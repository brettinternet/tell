package cli_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Guard literal argv, prepared stdin, caller-input isolation, and private output.
func TestCommandNotify(t *testing.T) {
	base := environment(t)
	project, err := filepath.EvalSymlinks(base)
	if err != nil {
		t.Fatal(err)
	}
	record := provider(t, "tell-fixture")
	script := `#!/bin/sh
printf '%s\000' "$@" > "$FIXTURE_RECORD"
printf '%s\000' "$TELL_TITLE" "$TELL_TYPE" "$TELL_PROJECT" > "$FIXTURE_RECORD.env"
/bin/cat > "$FIXTURE_RECORD.body"
printf 'raw-provider-output credential://private@example.invalid\n'
printf 'raw-provider-output credential://private@example.invalid\n' >&2
exit "${FIXTURE_EXIT:-0}"
`
	if err := os.WriteFile(filepath.Join(filepath.Dir(record), "tell-fixture"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(base, ".tell.yaml")
	data := "notify:\n  command: [tell-fixture, 'two words', '', '$(no-shell); *', 'credential://private@example.invalid']\n"
	if err := os.WriteFile(private, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	in := stdin(t, "hook summary\n")
	t.Setenv("TELL_TITLE", "wrong title")
	t.Setenv("TELL_TYPE", "wrong type")
	for _, args := range [][]string{{}, {"--json", "config"}, {"--json", "config", "check"}} {
		code, out, errOut := run(args...)
		if code != 0 || strings.Contains(out+errOut, "credential://") {
			t.Fatalf("inspection: %d %s %s", code, out, errOut)
		}
		if _, err := os.Stat(record); !os.IsNotExist(err) {
			t.Fatal("inspection launched command")
		}
	}
	code, out, errOut := run("--json", "notify", "--via", "command", "--title", "title\x1b[31m", "--type", "warning", "body-sentinel\x1b[31m\nnext")
	assertResult(t, code, out, errOut, "command", "accepted", "")
	for suffix, want := range map[string]string{
		"":      "two words\x00\x00$(no-shell); *\x00credential://private@example.invalid\x00",
		".env":  "title\x00warning\x00" + project + "\x00",
		".body": "body-sentinel\nnext",
	} {
		got, err := os.ReadFile(record + suffix)
		if err != nil || string(got) != want {
			t.Fatalf("record %s: %q want %q (%v)", suffix, got, want, err)
		}
	}
	if pos, _ := in.Seek(0, io.SeekCurrent); pos != 0 {
		t.Fatal("command consumed caller stdin")
	}
	for _, tc := range []struct{ exit, outcome, reason string }{
		{"3", "not_sent", "no_match"}, {"1", "failed", "provider_failure"}, {"5", "failed", "provider_failure"},
	} {
		t.Setenv("FIXTURE_EXIT", tc.exit)
		code, out, errOut := run("--json", "--via", "command", "notify", "body-sentinel")
		assertResult(t, code, out, errOut, "command", tc.outcome, tc.reason)
	}
	t.Setenv("PATH", t.TempDir())
	code, out, errOut = run("--json", "--via", "command", "notify", "body-sentinel")
	assertResult(t, code, out, errOut, "command", "not_sent", "missing_executable")
	if err := os.WriteFile(private, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = run("--json", "--via", "command", "notify", "body-sentinel")
	assertResult(t, code, out, errOut, "command", "config", "invalid_config")
}
