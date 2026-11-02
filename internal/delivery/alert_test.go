package delivery

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// Only an explicit click may report acknowledged; anything else must let a
// cascade escalate or report failure.
func TestAlertResultParsing(t *testing.T) {
	for name, test := range map[string]struct {
		code           int
		stdout, stderr string
		outcome        string
		reason         string
	}{
		"clicked":           {0, "button returned:Acknowledge, gave up:false\n", "", "accepted", ""},
		"gave up":           {0, "button returned:, gave up:true\n", "", "not_sent", "unacknowledged"},
		"cancelled":         {1, "", "execution error: User canceled. (-128)", "not_sent", "unacknowledged"},
		"no gui":            {1, "", "execution error: No user interaction allowed. (-1713)", "not_sent", "no_user_interaction"},
		"other error":       {1, "", "execution error: something (-1728)", "failed", "provider_failure"},
		"unexpected output": {0, "button returned:Acknowledge", "", "failed", "provider_failure"},
	} {
		result := dialogResult(test.code, []byte(test.stdout), []byte(test.stderr))
		if result.Outcome != test.outcome || resultReason(result) != test.reason || result.Acknowledged != (test.outcome == "accepted") {
			t.Errorf("dialog %s: %+v", name, result)
		}
	}
	for name, test := range map[string]struct {
		code    int
		stdout  string
		outcome string
	}{
		"action":           {0, "42\nack\n", "accepted"},
		"closed":           {0, "42\n", "not_sent"},
		"id is not action": {0, "ack\n", "not_sent"},
		"failure":          {1, "42\nack\n", "failed"},
	} {
		result := actionResult(test.code, test.stdout)
		if result.Outcome != test.outcome || result.Acknowledged != (test.outcome == "accepted") {
			t.Errorf("notify-send %s: %+v", name, result)
		}
	}
	for version, want := range map[string]bool{"notify-send 0.7.9": false, "notify-send 0.7.10": true, "notify-send 0.8.3": true, "garbage": false} {
		if notifySendSupportsActions(version) != want {
			t.Errorf("version %q", version)
		}
	}
}

// A human wait longer than --timeout must not starve later cascade steps, and
// message text reaches the provider only as argv.
func TestAlertWaitOutsideTimeoutBudget(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	t.Setenv("PATH", dir)
	t.Setenv("FIXTURE_RECORD", record)
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nprintf '%s\\000' \"$@\" > \"$FIXTURE_RECORD\"\n/bin/sleep 0.3\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	write("osascript", "echo 'button returned:, gave up:true'")
	write("notify-send", "echo 7")
	cfg := config.Config{Timeout: "100ms", Notify: config.Notify{Via: "alert", Steps: []config.NotifyStep{{Via: "alert", Wait: "1s"}, {Via: "stderr"}}}, Secrets: config.Secrets{Policy: "off"}}
	message := Message{Title: "-title ' \" \\\nnext", Body: "--body ' \" \\\n$(no-shell)", Type: "info"}
	// Exclude first-use secret-rule initialization from this provider-wait test.
	// Under the race detector it can consume the entire 100ms delivery budget.
	if _, _, reason := PrepareMessage(message, cfg.Secrets.Policy); reason != "" {
		t.Fatalf("prepare: %s", reason)
	}
	stderr := &strings.Builder{}
	result := Notify(context.Background(), cfg, message, stderr)
	if result.Outcome != "accepted" || result.Route != "stderr" || len(result.Attempts) != 2 || result.Attempts[0].Reason != "unacknowledged" {
		t.Fatalf("cascade: %+v", result)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	want := []string{"--", message.Title, message.Body}
	if runtime.GOOS == "darwin" {
		want = append(want, "1")
	}
	if !reflect.DeepEqual(got[len(got)-len(want):], want) {
		t.Fatalf("argv: %q", got)
	}

	// Without a wait, the same provider exceeds the budget.
	result = notifyAlert(context.Background(), 100*time.Millisecond, 0, message, "darwin")
	if result.Outcome != "uncertain" {
		t.Fatalf("bounded provider: %+v", result)
	}
}
