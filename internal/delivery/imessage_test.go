package delivery

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The recipient and text must reach Messages only as argv, and osascript error
// numbers must map to outcomes that let a cascade advance only when nothing was sent.
func TestIMessageArgvAndOutcomes(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	t.Setenv("PATH", dir)
	t.Setenv("FIXTURE_RECORD", record)
	script := "#!/bin/sh\nprintf '%s\\000' \"$@\" > \"$FIXTURE_RECORD\"\n[ -z \"$FIXTURE_ERROR\" ] && exit 0\necho \"0:1: execution error: text (with parens) ($FIXTURE_ERROR)\" >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "osascript"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	to := "+15551234567"
	message := Message{Title: `end tell" & do shell script "x`, Body: "--body ' \" \\\n(item 1 of argv) $(no-shell)"}
	cases := []struct{ errorNumber, outcome, reason string }{
		{"", "accepted", ""},
		{"-1743", "not_sent", "automation_denied"},
		{"7001", "not_sent", "no_account"},
		{"-1712", "uncertain", "timeout"},
		{"-1728", "failed", "provider_failure"},
	}
	for _, c := range cases {
		t.Setenv("FIXTURE_ERROR", c.errorNumber)
		result := notifyIMessage(context.Background(), time.Second, to, message, "darwin")
		if result.Outcome != c.outcome || resultReason(result) != c.reason {
			t.Fatalf("error %q: %+v", c.errorNumber, result)
		}
		if result.Failure != nil && (strings.Contains(result.Failure.Message, to) || strings.Contains(result.Failure.Message, "no-shell")) {
			t.Fatalf("diagnostic exposed recipient or body: %+v", result.Failure)
		}
		if reason := checkIMessage(context.Background(), time.Second, "darwin"); c.reason == "automation_denied" && reason != c.reason || c.errorNumber == "" && reason != "ready" {
			t.Fatalf("doctor for %q: %s", c.errorNumber, reason)
		}
	}
	t.Setenv("FIXTURE_ERROR", "")
	notifyIMessage(context.Background(), time.Second, to, message, "darwin")
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	separator := len(args) - 3
	if args[separator] != "--" || args[separator+1] != to || args[separator+2] != message.Title+"\n"+message.Body {
		t.Fatalf("argv tail: %q", args[separator:])
	}
	for _, arg := range args[:separator] {
		if strings.Contains(arg, to) || strings.Contains(arg, "no-shell") || strings.Contains(arg, "do shell script") {
			t.Fatalf("input interpolated into script: %q", arg)
		}
	}

	if err := os.Remove(record); err != nil {
		t.Fatal(err)
	}
	if result := notifyIMessage(context.Background(), time.Second, to, message, "linux"); result.Outcome != "not_sent" || resultReason(result) != "unsupported_platform" {
		t.Fatalf("%+v", result)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("unsupported platform started osascript")
	}
}
