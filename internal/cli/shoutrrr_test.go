package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/delivery"
)

const shoutrrrToken = "private-shoutrrr-token"

// webhook serves a generic Shoutrrr target. The token travels in a header, so
// any leak of the URL or request configuration shows up as the token.
func webhook(t *testing.T, status int, delay time.Duration) (string, *atomic.Int32, *atomic.Value) {
	t.Helper()
	var calls atomic.Int32
	var body atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		data, _ := io.ReadAll(r.Body)
		if r.Header.Get("Authorization") == "Bearer "+shoutrrrToken {
			body.Store(string(data))
		}
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
		w.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	return "generic://" + strings.TrimPrefix(server.URL, "http://") + "/hook?disabletls=yes&template=json&@Authorization=Bearer+" + shoutrrrToken, &calls, &body
}

func assertNoShoutrrrLeak(t *testing.T, texts ...string) {
	t.Helper()
	for _, text := range texts {
		for _, secret := range []string{shoutrrrToken, "generic://", "127.0.0.1", "body-sentinel"} {
			if strings.Contains(text, secret) {
				t.Fatalf("leaked %q: %s", secret, text)
			}
		}
	}
}

// Outcome mapping decides whether callers retry or fall back; a timeout after
// sending must stay uncertain so it never duplicates a delivered notification.
func TestShoutrrrOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name, urls             string
		status                 int
		delay                  time.Duration
		code                   int
		outcome, reason        string
		wantCalls, wantDeliver bool
	}{
		{name: "accepted", status: 200, code: 0, outcome: "accepted", wantCalls: true, wantDeliver: true},
		{name: "server error", status: 500, code: 4, outcome: "failed", reason: "provider_failure", wantCalls: true},
		{name: "slow server", status: 200, delay: 5 * time.Second, code: 5, outcome: "uncertain", reason: "timeout", wantCalls: true},
		// Transport errors quote the request URL; only a fixed hint may surface.
		{name: "connection refused", urls: "generic://127.0.0.1:1/hook?disabletls=yes&@Authorization=Bearer+" + shoutrrrToken, code: 4, outcome: "failed", reason: "provider_failure"},
		{name: "invalid url", urls: "generic://" + shoutrrrToken + "@[::1:bad/hook", code: 3, outcome: "not_sent", reason: "invalid_config"},
		{name: "unknown service", urls: "nosuchservice://" + shoutrrrToken + "@example.invalid", code: 3, outcome: "not_sent", reason: "invalid_config"},
		{name: "empty env", urls: " ", code: 3, outcome: "not_sent", reason: "missing_urls"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment(t)
			url, calls, body := webhook(t, tc.status, tc.delay)
			if tc.urls != "" {
				url = tc.urls
			}
			t.Setenv("PHONE_URLS", url)
			writeUserConfig(t, "timeout: 500ms\nnotify:\n  via: shoutrrr\nshoutrrr:\n  urls_env: PHONE_URLS\n")
			code, output, diagnostics := run("--json", "notify", "--title", "Build", "body-sentinel")
			var result delivery.Result
			if json.Unmarshal([]byte(output), &result) != nil || code != tc.code || result.Outcome != tc.outcome || result.Route != "shoutrrr" {
				t.Fatalf("result: code=%d %s %s", code, output, diagnostics)
			}
			if tc.reason != "" && (result.Failure == nil || result.Failure.Reason != tc.reason) {
				t.Fatalf("reason: %s", output)
			}
			if tc.name == "connection refused" && !strings.Contains(result.Failure.Message, "connection error") {
				t.Fatalf("missing connection hint: %s", output)
			}
			if len(result.Attempts) != 1 || result.Attempts[0].Label != "shoutrrr:PHONE_URLS" {
				t.Fatalf("attempts: %s", output)
			}
			if (calls.Load() > 0) != tc.wantCalls {
				t.Fatalf("calls=%d", calls.Load())
			}
			if tc.wantDeliver {
				got, _ := body.Load().(string)
				var payload map[string]string
				if json.Unmarshal([]byte(got), &payload) != nil || payload["title"] != "Build" || payload["message"] != "body-sentinel" {
					t.Fatalf("payload: %q", got)
				}
			}
			// Human output for failures goes to stderr; it must stay URL-free too.
			_, humanOut, humanErr := run("notify", "body-sentinel")
			assertNoShoutrrrLeak(t, strings.ReplaceAll(output, `"body-sentinel"`, ""), diagnostics, humanOut, humanErr)
		})
	}
}

// Shoutrrr is the built-in fallback for hosts without Apprise installed.
func TestShoutrrrCascadeAfterMissingApprise(t *testing.T) {
	environment(t)
	t.Setenv("PATH", t.TempDir())
	url, calls, _ := webhook(t, 200, 0)
	t.Setenv("PHONE_URLS", url)
	writeUserConfig(t, "notify:\n  via:\n    - apprise: {tag: phone}\n    - shoutrrr: {urls_env: PHONE_URLS}\n    - stderr\n")
	code, output, diagnostics := run("--json", "notify", "body-sentinel")
	var result delivery.Result
	if json.Unmarshal([]byte(output), &result) != nil || code != 0 || result.Outcome != "accepted" || calls.Load() != 1 || len(result.Attempts) != 2 {
		t.Fatalf("cascade: code=%d %s %s", code, output, diagnostics)
	}
	if a := result.Attempts[0]; a.Label != "apprise:phone" || a.Outcome != "not_sent" || a.Reason != "missing_executable" {
		t.Fatalf("first attempt: %+v", a)
	}
	if a := result.Attempts[1]; a.Label != "shoutrrr:PHONE_URLS" || a.Outcome != "accepted" {
		t.Fatalf("second attempt: %+v", a)
	}
	assertNoShoutrrrLeak(t, output, diagnostics)
}

// Doctor parses URLs to report readiness but must never send or leak them.
func TestShoutrrrDoctor(t *testing.T) {
	for _, tc := range []struct {
		name, urls, reason string
		set, ready         bool
	}{
		{name: "ready", set: true, ready: true, reason: "ready"},
		{name: "missing env", reason: "missing_urls"},
		{name: "invalid url", urls: "generic://" + shoutrrrToken + "@[::1:bad/hook", set: true, reason: "invalid_config"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment(t)
			url, calls, _ := webhook(t, 200, 0)
			if tc.urls != "" {
				url = tc.urls
			}
			if tc.set {
				t.Setenv("PHONE_URLS", url)
			}
			code, output, diagnostics := run("--json", "doctor", "--via", "shoutrrr", "--shoutrrr-urls-env", "PHONE_URLS")
			var report struct {
				Ready  bool                 `json:"ready"`
				Steps  []delivery.Readiness `json:"steps"`
				Routes []delivery.Readiness `json:"routes"`
			}
			if json.Unmarshal([]byte(output), &report) != nil || report.Ready != tc.ready || (code == 0) != tc.ready || len(report.Routes) == 0 {
				t.Fatalf("doctor: code=%d %s %s", code, output, diagnostics)
			}
			if r := report.Routes[0]; r.Route != "shoutrrr" || r.Reason != tc.reason || r.Label != "shoutrrr:PHONE_URLS" {
				t.Fatalf("readiness: %+v", r)
			}
			_, humanOut, humanErr := run("doctor", "--via", "shoutrrr", "--shoutrrr-urls-env", "PHONE_URLS")
			if !tc.ready && !strings.Contains(humanOut, "Shoutrrr") {
				t.Fatalf("human reason: %s", humanOut)
			}
			assertNoShoutrrrLeak(t, output, diagnostics, humanOut, humanErr)
			if calls.Load() != 0 {
				t.Fatal("doctor sent a notification")
			}
		})
	}
}

// The URL is a credential, so shared project config must not select it.
func TestShoutrrrURLsEnvIsPersonal(t *testing.T) {
	base := environment(t)
	if err := os.WriteFile(filepath.Join(base, "tell.yaml"), []byte("shoutrrr:\n  urls_env: PHONE_URLS\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, diagnostics := run("config", "check"); code == 0 || !strings.Contains(diagnostics, "shoutrrr.urls_env is personal routing") {
		t.Fatalf("shared shoutrrr accepted: %d %s", code, diagnostics)
	}
}
