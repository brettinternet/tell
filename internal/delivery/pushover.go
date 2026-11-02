package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// pushoverAPI is a test seam, never a user setting.
var pushoverAPI = "https://api.pushover.net/1"

var pushoverClient = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

var (
	pushoverKey      = regexp.MustCompile(`^[A-Za-z0-9]{30}$`)
	pushoverReceipt  = regexp.MustCompile(`^[A-Za-z0-9]{1,64}$`)
	pushoverTagValue = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)
)

// Pushover limits are in characters, below Tell's byte limits.
const (
	pushoverTitleLimit   = 250
	pushoverMessageLimit = 1024
)

// ValidPushoverReceipt, ValidPushoverTag, and ValidPushoverCallback check
// per-call values before any request. Tags and receipts become URL path
// segments, so dot segments and separators are refused.
func ValidPushoverReceipt(receipt string) bool { return pushoverReceipt.MatchString(receipt) }
func ValidPushoverTag(tag string) bool         { return pushoverTagValue.MatchString(tag) }
func ValidPushoverCallback(callback string) bool {
	u, err := url.Parse(callback)
	return err == nil && len(callback) <= 512 && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}

// pushoverCredentials reads the named variables. The values are credentials and
// never leave this package except in request bodies.
func pushoverCredentials(settings config.Pushover, needUser bool) (token, user, reason string) {
	token = os.Getenv(settings.TokenEnv)
	if needUser {
		user = os.Getenv(settings.UserEnv)
	}
	switch {
	case settings.TokenEnv == "" || token == "" || needUser && (settings.UserEnv == "" || user == ""):
		return "", "", "missing_credentials"
	case !pushoverKey.MatchString(token) || needUser && !pushoverKey.MatchString(user):
		return "", "", "invalid_credentials"
	}
	return token, user, ""
}

// PushoverReadiness checks the credential variables without network calls.
func PushoverReadiness(settings config.Pushover) string {
	if _, _, reason := pushoverCredentials(settings, true); reason != "" {
		return reason
	}
	return "ready"
}

func credentialFailure(reason string) *Result {
	message := "Pushover token or user key environment variable is unset or empty"
	if reason == "invalid_credentials" {
		message = "Pushover token or user key is not a 30-character key"
	}
	return NewResult("pushover", "not_sent", "invalid_config", message+"; run tell doctor --via pushover")
}

type pushoverResponse struct {
	Status       int    `json:"status"`
	Receipt      string `json:"receipt"`
	Acknowledged int    `json:"acknowledged"`
	Expired      int    `json:"expired"`
}

// pushoverCall sends one request and maps the response. Errors from net/http
// embed the URL, which can hold the token, so only fixed messages are returned.
func pushoverCall(ctx context.Context, timeout time.Duration, method, path string, form url.Values) (*pushoverResponse, *Result) {
	const route = "pushover"
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var wrote atomic.Bool
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{WroteRequest: func(httptrace.WroteRequestInfo) { wrote.Store(true) }})
	endpoint, body := pushoverAPI+path, io.Reader(nil)
	if method == http.MethodGet {
		endpoint += "?" + form.Encode()
	} else {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, NewResult(route, "config", "invalid_config", "Pushover request could not be built")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	interrupted := func(sent bool) *Result {
		reason := "cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = "timeout"
		}
		if !sent {
			return NewResult(route, "not_sent", reason, "Pushover request did not start before the deadline")
		}
		return NewResult(route, "uncertain", reason, "Pushover request was interrupted; it may have been delivered. Check the Pushover app before retrying")
	}
	resp, err := pushoverClient.Do(req)
	if err != nil {
		switch {
		case ctx.Err() != nil:
			return nil, interrupted(wrote.Load())
		case !wrote.Load():
			return nil, NewResult(route, "not_sent", "start_failed", "could not connect to Pushover; check network access")
		default:
			return nil, NewResult(route, "uncertain", "provider_failure", "Pushover connection failed after the request was sent; it may have been delivered. Check the Pushover app before retrying")
		}
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, NewResult(route, "not_sent", "rate_limited", "Pushover rate limit reached")
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, NewResult(route, "not_sent", "invalid_config", "Pushover rejected the request; check the token, user key, and options")
	case resp.StatusCode != http.StatusOK:
		return nil, NewResult(route, "failed", "provider_failure", "Pushover reported a server error (HTTP "+strconv.Itoa(resp.StatusCode)+")")
	}
	const limit = 64 * 1024
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if ctx.Err() != nil {
		return nil, interrupted(true)
	}
	var parsed pushoverResponse
	if err != nil || len(raw) > limit || json.Unmarshal(raw, &parsed) != nil || parsed.Status != 1 {
		return nil, NewResult(route, "uncertain", "invalid_response", "Pushover returned an unreadable response; check the Pushover app before retrying")
	}
	return &parsed, nil
}

func notifyPushover(ctx context.Context, timeout time.Duration, settings config.Pushover, message Message) *Result {
	const route = "pushover"
	token, user, reason := pushoverCredentials(settings, true)
	if reason != "" {
		return credentialFailure(reason)
	}
	form := url.Values{"token": {token}, "user": {user}, "message": {truncateRunes(message.Body, pushoverMessageLimit)}}
	if message.Title != "" {
		form.Set("title", truncateRunes(message.Title, pushoverTitleLimit))
	}
	if settings.Priority != "" {
		form.Set("priority", config.PushoverPriorities[settings.Priority])
	}
	if settings.Device != "" {
		form.Set("device", settings.Device)
	}
	if settings.Emergency() {
		retry, _ := time.ParseDuration(settings.Retry) // validated configuration
		expire, _ := time.ParseDuration(settings.Expire)
		form.Set("retry", strconv.Itoa(int(retry.Seconds())))
		form.Set("expire", strconv.Itoa(int(expire.Seconds())))
		if message.PushoverTag != "" {
			form.Set("tags", message.PushoverTag)
		}
		if message.PushoverCallback != "" {
			form.Set("callback", message.PushoverCallback)
		}
	}
	response, failure := pushoverCall(ctx, timeout, http.MethodPost, "/messages.json", form)
	if failure != nil {
		return failure
	}
	result := NewResult(route, "accepted", "", "")
	if settings.Emergency() {
		// Later tiers need the receipt; without it acceptance cannot be checked.
		if !ValidPushoverReceipt(response.Receipt) {
			return NewResult(route, "uncertain", "invalid_response", "Pushover accepted the alert without a usable receipt; check the Pushover app")
		}
		result.Receipt = response.Receipt
	}
	return result
}

// ReceiptStatus is Pushover's acknowledgement state for one emergency alert.
type ReceiptStatus struct {
	Acknowledged bool `json:"acknowledged"`
	Expired      bool `json:"expired"`
}

// PushoverStatus reads one receipt once. Tell never polls.
func PushoverStatus(ctx context.Context, cfg config.Config, receipt string) (ReceiptStatus, *Result) {
	token, _, reason := pushoverCredentials(config.PushoverSettings(cfg), false)
	if reason != "" {
		return ReceiptStatus{}, credentialFailure(reason)
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	response, failure := pushoverCall(ctx, timeout, http.MethodGet, "/receipts/"+receipt+".json", url.Values{"token": {token}})
	if failure != nil {
		return ReceiptStatus{}, failure
	}
	return ReceiptStatus{Acknowledged: response.Acknowledged == 1, Expired: response.Expired == 1}, nil
}

// PushoverCancel stops the app's active emergency alerts carrying tag.
func PushoverCancel(ctx context.Context, cfg config.Config, tag string) *Result {
	token, _, reason := pushoverCredentials(config.PushoverSettings(cfg), false)
	if reason != "" {
		return credentialFailure(reason)
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	if _, failure := pushoverCall(ctx, timeout, http.MethodPost, "/receipts/cancel_by_tag/"+tag+".json", url.Values{"token": {token}}); failure != nil {
		return failure
	}
	return NewResult("pushover", "accepted", "", "")
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-len(truncationMarker)]) + truncationMarker
}
