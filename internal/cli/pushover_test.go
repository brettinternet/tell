package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/delivery"
)

const (
	pushoverToken = "tokentokentokentokentokentoken"
	pushoverUser  = "useruseruseruseruseruseruserus"
)

// Doctor and bare inspection read the credential variables but must never
// print them or contact Pushover.
func TestPushoverInspectionNeverLeaksCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, token, reason string
		ready               bool
	}{
		{name: "ready", token: pushoverToken, reason: "ready", ready: true},
		{name: "missing", reason: "missing_credentials"},
		{name: "malformed", token: pushoverToken + "-x", reason: "invalid_credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment(t)
			t.Setenv("PO_TOKEN", tc.token)
			t.Setenv("PO_USER", pushoverUser)
			writeUserConfig(t, "notify: {via: pushover}\npushover: {token_env: PO_TOKEN, user_env: PO_USER, priority: emergency, retry: 1m, expire: 1h}\n")
			code, output, diagnostics := run("--json", "doctor")
			var report struct {
				Routes []delivery.Readiness `json:"routes"`
			}
			if json.Unmarshal([]byte(output), &report) != nil || (code == 0) != tc.ready || len(report.Routes) != 1 || report.Routes[0].Reason != tc.reason {
				t.Fatalf("doctor: code=%d %s %s", code, output, diagnostics)
			}
			_, humanOut, humanErr := run("doctor")
			bareCode, bareOut, bareErr := run("pushover")
			if bareCode != 0 || !strings.Contains(bareOut, "status") {
				t.Fatalf("bare pushover: %d %s %s", bareCode, bareOut, bareErr)
			}
			for _, text := range []string{output, diagnostics, humanOut, humanErr, bareOut, bareErr} {
				if strings.Contains(text, pushoverToken) || strings.Contains(text, pushoverUser) {
					t.Fatalf("leaked credential: %s", text)
				}
			}
		})
	}
}

// Credential variable names select a personal account, so shared project
// config must not set them; emergency alerts must bound their repeats.
func TestPushoverConfigValidation(t *testing.T) {
	base := environment(t)
	if err := os.WriteFile(filepath.Join(base, "tell.yaml"), []byte("pushover:\n  token_env: PO_TOKEN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, diagnostics := run("config", "check"); code == 0 || !strings.Contains(diagnostics, "pushover.token_env is personal routing") {
		t.Fatalf("shared pushover accepted: %d %s", code, diagnostics)
	}

	environment(t)
	writeUserConfig(t, "notify:\n  via:\n    - pushover: {token_env: PO_TOKEN, user_env: PO_USER, priority: emergency}\n")
	if code, _, diagnostics := run("config", "check"); code == 0 || !strings.Contains(diagnostics, "requires retry and expire") {
		t.Fatalf("unbounded emergency accepted: %d %s", code, diagnostics)
	}
}

// A tag the caller cannot later cancel must fail before anything is sent.
func TestPushoverPerCallOptionsNeedEmergency(t *testing.T) {
	environment(t)
	t.Setenv("PO_TOKEN", pushoverToken)
	t.Setenv("PO_USER", pushoverUser)
	writeUserConfig(t, "notify: {via: pushover}\npushover: {token_env: PO_TOKEN, user_env: PO_USER, priority: high}\n")
	for _, args := range [][]string{
		{"--json", "notify", "--pushover-tag", "inc-1", "body"},
		{"--json", "notify", "--pushover-tag", "../x", "body"},
		{"--json", "pushover", "status", "../x"},
		{"--json", "pushover", "cancel", "--tag", "a/b"},
	} {
		code, output, diagnostics := run(args...)
		if code != 2 || !strings.Contains(output, `"usage"`) {
			t.Fatalf("%v: code=%d %s %s", args, code, output, diagnostics)
		}
	}
}
