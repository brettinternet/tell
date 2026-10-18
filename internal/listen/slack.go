package listen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

// SlackOptions contains invocation-owned streams and credentials, never printable configuration.
type SlackOptions struct {
	Token    string
	Timeout  time.Duration
	Out, Err io.Writer
}
type transport struct {
	endpoint      string
	client        *http.Client
	dial          func(context.Context, string) (*websocket.Conn, error)
	wait          func(context.Context, time.Duration) bool
	now           func() time.Time
	jitter        func(time.Duration) time.Duration
	outputTimeout time.Duration
}

func production() transport {
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return transport{
		endpoint: "https://slack.com/api/apps.connections.open", client: client,
		dial: func(ctx context.Context, address string) (*websocket.Conn, error) {
			u, err := url.Parse(address)
			if err != nil || u.Scheme != "wss" || u.Host == "" || u.User != nil {
				return nil, &setupError{reason: "invalid_connection_url"}
			}
			conn, response, err := websocket.Dial(ctx, address, &websocket.DialOptions{HTTPClient: client})
			if err != nil && response != nil {
				if response.StatusCode == http.StatusTooManyRequests {
					return nil, &setupError{reason: "rate_limited", retry: true, after: retryAfter(response.Header.Get("Retry-After"), time.Now())}
				}
				if response.StatusCode >= 400 && response.StatusCode < 500 {
					return nil, &setupError{reason: "websocket_setup_failed"}
				}
			}
			return conn, err
		}, wait: wait, now: time.Now, jitter: func(d time.Duration) time.Duration { return d + time.Duration(rand.Int64N(int64(d/5)+1)) }, outputTimeout: 5 * time.Second,
	}
}
func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Slack captures until cancellation or a fatal source/output failure. One blocked
// writer goroutine per stream is the maximum; process exit releases blocked OS IO.
// Neither the provider reader nor acknowledgments wait for these streams.
func Slack(ctx context.Context, opts SlackOptions) error { return run(ctx, opts, production()) }
func run(ctx context.Context, opts SlackOptions, tr transport) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a := newAdmission()
	diagnostics := make(chan []byte, 64)
	failures := make(chan error, 1)
	fail := func(err error) {
		select {
		case failures <- err:
		default:
		}
	}
	receipt := func(e Envelope, outcome string) {
		line := fmt.Sprintf("tell listen: source=slack outcome=%s\n", outcome)
		if e.ID != "" {
			line = fmt.Sprintf("tell listen: source=slack id=%q type=%q outcome=%s\n", e.ID, e.Type, outcome)
		}
		select {
		case diagnostics <- []byte(line):
		default:
			fail(errors.New("listener diagnostic output busy"))
		}
	}
	outputCtx, stopOutput := context.WithCancel(context.Background())
	defer stopOutput()
	outputDone := make(chan struct{}, 2)
	go func() { output(outputCtx, opts.Out, a.events, tr.outputTimeout, fail); outputDone <- struct{}{} }()
	go func() { output(outputCtx, opts.Err, diagnostics, tr.outputTimeout, fail); outputDone <- struct{}{} }()
	sourceDone := make(chan error, 1)
	go func() { sourceDone <- receive(ctx, opts, tr, a, receipt) }()
	var result error
	select {
	case <-ctx.Done():
		cancel()
		<-sourceDone
	case result = <-failures:
		cancel()
		<-sourceDone
	case result = <-sourceDone:
		if ctx.Err() != nil {
			result = nil
		}
	}
	// Stop admission first, then finish bounded writes. This preserves stopped
	// receipts and prevents successful writes racing the caller after return.
	cancel()
	if result != nil {
		receipt(Envelope{}, "failed reason="+strconv.Quote(result.Error()))
	}
	close(a.events)
	close(diagnostics)
	<-outputDone
	<-outputDone
	select {
	case err := <-failures:
		if result == nil {
			result = err
		}
	default:
	}
	return result
}
func output(ctx context.Context, writer io.Writer, queue <-chan []byte, timeout time.Duration, fail func(error)) {
	// A dedicated writer bounds the number of blocked Writes, even for arbitrary
	// io.Writers without deadlines. Never spawn another after a timeout.
	jobs := make(chan []byte)
	defer close(jobs)
	done := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-jobs:
				if !ok {
					return
				}
				n, err := writer.Write(data)
				if err == nil && n != len(data) {
					err = io.ErrShortWrite
				}
				select {
				case done <- err:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-queue:
			if !ok {
				return
			}
			select {
			case jobs <- data:
			case <-ctx.Done():
				return
			}
			timer := time.NewTimer(timeout)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case err := <-done:
				timer.Stop()
				if err != nil {
					fail(errors.New("listener output failed"))
					return
				}
			case <-timer.C:
				fail(errors.New("listener output stalled"))
				return
			}
		}
	}
}

type setupError struct {
	reason string
	retry  bool
	after  time.Duration
}

func (e *setupError) Error() string { return "Slack setup: " + e.reason }
func open(ctx context.Context, opts SlackOptions, tr transport) (string, *setupError) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tr.endpoint, nil)
	if err != nil {
		return "", &setupError{reason: "invalid_setup"}
	}
	req.Header.Set("Authorization", "Bearer "+opts.Token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := tr.client.Do(req)
	if err != nil {
		return "", &setupError{reason: "network", retry: true}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 {
		return "", &setupError{reason: "rate_limited", retry: true, after: retryAfter(resp.Header.Get("Retry-After"), tr.now())}
	}
	if resp.StatusCode >= 500 {
		return "", &setupError{reason: "server_error", retry: true}
	}
	if resp.StatusCode != 200 {
		return "", &setupError{reason: "http_error"}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, rawLimit+1))
	if err != nil {
		return "", &setupError{reason: "network", retry: true}
	}
	data, err := object(raw)
	if err != nil {
		return "", &setupError{reason: "invalid_response"}
	}
	var ok bool
	_ = json.Unmarshal(data["ok"], &ok)
	if !ok {
		reason := field(data, "error")
		switch reason {
		case "invalid_auth", "not_authed", "token_revoked", "account_inactive", "missing_scope", "not_allowed_token_type", "app_not_enabled":
		case "ratelimited":
			return "", &setupError{reason: "rate_limited", retry: true, after: retryAfter(resp.Header.Get("Retry-After"), tr.now())}
		case "internal_error", "fatal_error", "service_unavailable":
			return "", &setupError{reason: "server_error", retry: true}
		default:
			reason = "setup_failed"
		}
		return "", &setupError{reason: reason}
	}
	address := field(data, "url")
	if address == "" {
		return "", &setupError{reason: "invalid_response"}
	}
	return address, nil
}
func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseUint(value, 10, 32); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return date.Sub(now)
	}
	return 0
}
func receive(ctx context.Context, opts SlackOptions, tr transport, a *admission, receipt func(Envelope, string)) error {
	defer receipt(Envelope{}, "stopped")
	delay := time.Second
	for ctx.Err() == nil {
		receipt(Envelope{}, "connecting")
		address, failure := open(ctx, opts, tr)
		retryAfter := time.Duration(0)
		if failure != nil {
			if !failure.retry {
				return failure
			}
			retryAfter = failure.after
		} else {
			setupCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
			conn, err := tr.dial(setupCtx, address)
			cancel()
			if err == nil {
				started := tr.now()
				receipt(Envelope{}, "connected")
				err = connection(ctx, conn, tr, a, receipt)
				conn.CloseNow()
				if err != nil {
					return err
				}
				if tr.now().Sub(started) >= time.Minute {
					delay = time.Second
				}
			} else {
				var setup *setupError
				if errors.As(err, &setup) {
					if !setup.retry {
						return setup
					}
					retryAfter = setup.after
				}
			}
		}
		if ctx.Err() != nil {
			break
		}
		receipt(Envelope{}, "reconnecting")
		pause := tr.jitter(delay)
		if pause > time.Minute {
			pause = time.Minute
		}
		if retryAfter > pause {
			pause = retryAfter
		}
		if !tr.wait(ctx, pause) {
			break
		}
		delay *= 2
		if delay > time.Minute {
			delay = time.Minute
		}
	}
	return nil
}
func connection(ctx context.Context, conn *websocket.Conn, tr transport, a *admission, receipt func(Envelope, string)) error {
	conn.SetReadLimit(rawLimit)
	for {
		kind, raw, err := conn.Read(ctx)
		if err != nil {
			return nil
		}
		if kind != websocket.MessageText {
			receipt(Envelope{}, "not_sent/invalid_frame")
			continue
		}
		message, err := frame(raw)
		if err != nil {
			receipt(Envelope{}, "not_sent/invalid_json")
			continue
		}
		if id := field(message, "envelope_id"); metadata(id, 256) {
			ack, _ := json.Marshal(map[string]string{"envelope_id": id})
			ackCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err = conn.Write(ackCtx, websocket.MessageText, ack)
			cancel()
			if err != nil {
				return nil
			}
		} else if field(message, "type") != "hello" && field(message, "type") != "disconnect" {
			receipt(Envelope{}, "not_sent/invalid_identity")
			continue
		}
		switch field(message, "type") {
		case "hello":
			continue
		case "disconnect":
			if field(message, "reason") == "link_disabled" {
				return errors.New("Slack disconnected: link_disabled")
			}
			return nil
		case "events_api":
			e, encoded, err := event(message["payload"], tr.now())
			if errors.Is(err, errIgnored) {
				receipt(Envelope{}, "ignored")
				continue
			}
			if err != nil {
				receipt(Envelope{}, "not_sent/"+err.Error())
				continue
			}
			receipt(e, a.admit(e, encoded, tr.now()))
		default:
			receipt(Envelope{}, "ignored")
		}
	}
}
