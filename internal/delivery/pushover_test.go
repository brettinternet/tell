package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

const (
	pushoverTestToken   = "tokentokentokentokentokentoken"
	pushoverTestUser    = "useruseruseruseruseruseruserus"
	pushoverTestReceipt = "receiptsentinel0123456789abcde"
	pushoverTestHook    = "https://hooks.example/callback-sentinel"
)

type pushoverRequest struct {
	method, path string
	form         url.Values
}

// fakePushover records requests and replies with status and body after delay.
// The returned func snapshots the requests received so far.
func fakePushover(t *testing.T, status int, body string, delay time.Duration) func() []pushoverRequest {
	t.Helper()
	var mu sync.Mutex
	var requests []pushoverRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		requests = append(requests, pushoverRequest{r.Method, r.URL.Path, r.Form})
		mu.Unlock()
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	previous := pushoverAPI
	pushoverAPI = server.URL + "/1"
	t.Cleanup(func() { pushoverAPI = previous })
	t.Setenv("PO_TOKEN", pushoverTestToken)
	t.Setenv("PO_USER", pushoverTestUser)
	return func() []pushoverRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]pushoverRequest(nil), requests...)
	}
}

func pushoverConfig(settings config.Pushover) config.Config {
	settings.TokenEnv, settings.UserEnv = "PO_TOKEN", "PO_USER"
	return config.Config{Timeout: "300ms", Notify: config.Notify{Via: "pushover"}, Pushover: settings}
}

func assertNoPushoverLeak(t *testing.T, result *Result) {
	t.Helper()
	data, _ := json.Marshal(result)
	text := string(data)
	if result.Receipt != "" {
		text = strings.ReplaceAll(text, `"receipt":"`+result.Receipt+`"`, "")
	}
	for _, secret := range []string{pushoverTestToken, pushoverTestUser, pushoverTestReceipt, pushoverTestHook, "body-sentinel", "127.0.0.1"} {
		if strings.Contains(text, secret) {
			t.Fatalf("leaked %q: %s", secret, text)
		}
	}
}

// Emergency requests must carry retry/expire/tags/callback so callers can
// cancel by tag; lower priorities must not, and only emergency returns a receipt.
func TestPushoverNotifyRequest(t *testing.T) {
	emergency := config.Pushover{Priority: "emergency", Retry: "2m", Expire: "1h", Device: "phone"}
	message := Message{Title: "Build", Body: "body-sentinel", PushoverTag: "inc-1", PushoverCallback: pushoverTestHook}

	requests := fakePushover(t, 200, `{"status":1,"request":"r","receipt":"`+pushoverTestReceipt+`"}`, 0)
	result := Notify(context.Background(), pushoverConfig(emergency), message, &strings.Builder{})
	if result.ExitCode() != 0 || result.Outcome != "accepted" || result.Receipt != pushoverTestReceipt || len(requests()) != 1 {
		t.Fatalf("emergency: %+v", result)
	}
	got := requests()[0]
	want := map[string]string{"token": pushoverTestToken, "user": pushoverTestUser, "title": "Build", "message": "body-sentinel", "priority": "2", "retry": "120", "expire": "3600", "device": "phone", "tags": "inc-1", "callback": pushoverTestHook}
	if got.method != http.MethodPost || got.path != "/1/messages.json" || len(got.form) != len(want) {
		t.Fatalf("emergency request: %+v", got)
	}
	for key, value := range want {
		if got.form.Get(key) != value {
			t.Fatalf("form %s = %q, want %q", key, got.form.Get(key), value)
		}
	}
	assertNoPushoverLeak(t, result)

	requests = fakePushover(t, 200, `{"status":1,"request":"r"}`, 0)
	result = Notify(context.Background(), pushoverConfig(config.Pushover{Priority: "high"}), message, &strings.Builder{})
	if result.Outcome != "accepted" || result.Receipt != "" || len(requests()) != 1 {
		t.Fatalf("high: %+v", result)
	}
	for _, key := range []string{"retry", "expire", "tags", "callback"} {
		if _, ok := requests()[0].form[key]; ok {
			t.Fatalf("non-emergency request sent %s", key)
		}
	}
	if requests()[0].form.Get("priority") != "1" {
		t.Fatalf("priority: %v", requests()[0].form)
	}
}

// Outcome mapping decides whether a cascade advances or a caller retries; an
// interrupted request must stay uncertain so it never duplicates an alert.
func TestPushoverNotifyOutcomes(t *testing.T) {
	emergency := config.Pushover{Priority: "emergency", Retry: "30s", Expire: "10m"}
	for _, tc := range []struct {
		name, body      string
		status          int
		delay           time.Duration
		settings        config.Pushover
		unsetUser, down bool
		outcome, reason string
		calls           int
	}{
		{name: "rejected", status: 400, body: `{"status":0,"errors":["token invalid"]}`, outcome: "not_sent", reason: "invalid_config", calls: 1},
		{name: "rate limited", status: 429, outcome: "not_sent", reason: "rate_limited", calls: 1},
		{name: "server error", status: 500, outcome: "failed", reason: "provider_failure", calls: 1},
		{name: "malformed", status: 200, body: "<html>", outcome: "uncertain", reason: "invalid_response", calls: 1},
		{name: "emergency without receipt", status: 200, body: `{"status":1}`, settings: emergency, outcome: "uncertain", reason: "invalid_response", calls: 1},
		{name: "slow", status: 200, body: `{"status":1}`, delay: 2 * time.Second, outcome: "uncertain", reason: "timeout", calls: 1},
		{name: "missing user key", unsetUser: true, outcome: "not_sent", reason: "invalid_config"},
		{name: "unreachable", down: true, outcome: "not_sent", reason: "start_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requests := fakePushover(t, tc.status, tc.body, tc.delay)
			if tc.unsetUser {
				t.Setenv("PO_USER", "")
			}
			if tc.down {
				pushoverAPI = "http://127.0.0.1:1/1"
			}
			result := Notify(context.Background(), pushoverConfig(tc.settings), Message{Body: "body-sentinel"}, &strings.Builder{})
			if result.Outcome != tc.outcome || result.Failure == nil || result.Failure.Reason != tc.reason || len(requests()) != tc.calls {
				t.Fatalf("result: %+v calls=%d", result, len(requests()))
			}
			assertNoPushoverLeak(t, result)
		})
	}
}

// Receipt status and cancel are single requests that send only the token.
func TestPushoverStatusAndCancel(t *testing.T) {
	cfg := pushoverConfig(config.Pushover{Priority: "emergency", Retry: "30s", Expire: "10m"})
	requests := fakePushover(t, 200, `{"status":1,"acknowledged":1,"expired":0}`, 0)
	status, failure := PushoverStatus(context.Background(), cfg, pushoverTestReceipt)
	if failure != nil || !status.Acknowledged || status.Expired || len(requests()) != 1 {
		t.Fatalf("status: %+v %+v", status, failure)
	}
	if r := requests()[0]; r.method != http.MethodGet || r.path != "/1/receipts/"+pushoverTestReceipt+".json" || r.form.Get("token") != pushoverTestToken || r.form.Has("user") {
		t.Fatalf("status request: %+v", r)
	}

	requests = fakePushover(t, 200, `{"status":1}`, 0)
	if result := PushoverCancel(context.Background(), cfg, "inc-1"); result.Outcome != "accepted" || len(requests()) != 1 {
		t.Fatalf("cancel: %+v", result)
	}
	if r := requests()[0]; r.method != http.MethodPost || r.path != "/1/receipts/cancel_by_tag/inc-1.json" || r.form.Get("token") != pushoverTestToken || len(r.form) != 1 {
		t.Fatalf("cancel request: %+v", r)
	}

	fakePushover(t, 404, `{"status":0}`, 0)
	if _, failure := PushoverStatus(context.Background(), cfg, pushoverTestReceipt); failure == nil || failure.Outcome != "not_sent" {
		t.Fatalf("unknown receipt: %+v", failure)
	} else {
		assertNoPushoverLeak(t, failure)
	}
}
