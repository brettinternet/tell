package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/brettinternet/tell/internal/cli"
	"github.com/brettinternet/tell/internal/config"
	"github.com/brettinternet/tell/internal/delivery"
)

// Real argv and stdin boundaries prevent accidental routing changes, shell
// interpretation, leaked secrets, and consumption of a Noon hook's JSON input.
func provider(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "args")
	script := `#!/bin/sh
printf '%s\000' "$@" > "$FIXTURE_RECORD"
if read -r input; then
  printf 'inherited stdin' > "$FIXTURE_RECORD.stdin"
fi
printf 'raw-provider-output credential://private@example.invalid\n'
printf 'raw-provider-output credential://private@example.invalid\n' >&2
if [ "$FIXTURE_WAIT" = yes ]; then
  exec /bin/sleep 30
fi
exit "${FIXTURE_EXIT:-0}"
`
	if err := os.WriteFile(filepath.Join(dir, "apprise"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("FIXTURE_RECORD", record)
	t.Setenv("FIXTURE_EXIT", "0")
	t.Setenv("FIXTURE_WAIT", "")
	t.Setenv("APPRISE_URLS", "credential://private@example.invalid")
	return record
}

func stdin(t *testing.T, text string) *os.File {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = original; file.Close() })
	return file
}

func assertResult(t *testing.T, code int, output, diagnostics, route, outcome, reason string) {
	t.Helper()
	var result delivery.Result
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid single object: %s: %v", output, err)
	}
	if result.SchemaVersion != 1 || result.Route != route || result.Outcome != outcome || result.ExitCode() != code {
		t.Fatalf("result: %d %s", code, output)
	}
	if reason != "" && (result.Failure == nil || result.Failure.Code != outcome || result.Failure.Reason != reason || result.Failure.Message == "") {
		t.Fatalf("failure: %s", output)
	}
	for _, secret := range []string{"body-sentinel", "credential://", "raw-provider-output"} {
		if strings.Contains(output+diagnostics, secret) {
			t.Fatalf("leaked %s: %s %s", secret, output, diagnostics)
		}
	}
}

func TestNotifyInputIsolation(t *testing.T) {
	environment(t)
	record := provider(t)
	in := stdin(t, "hook summary\n")
	code, out, errOut := run("notify", "literal")
	if code != 0 || out != "" || errOut != "literal\n" {
		t.Fatalf("stderr: %d %q %q", code, out, errOut)
	}
	if pos, _ := in.Seek(0, io.SeekCurrent); pos != 0 {
		t.Fatal("literal consumed stdin")
	}
	code, out, errOut = run("--json", "notify", "-")
	assertResult(t, code, out, "", "stderr", "accepted", "")
	if errOut != "hook summary\n\n" {
		t.Fatalf("stdin body: %q", errOut)
	}
	for _, args := range [][]string{
		{"--json", "--via", "apprise", "notify", ""},
		{"--json", "--via", "apprise", "notify", "-"},
		{"--json", "--via", "apprise", "notify", "  \n"},
	} {
		code, out, errOut = run(args...)
		assertResult(t, code, out, errOut, "apprise", "config", "empty_message")
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("stderr or empty input launched provider")
	}
}

func TestAppriseArguments(t *testing.T) {
	base := environment(t)
	record := provider(t)
	in := stdin(t, "hook summary\n")
	project := filepath.Join(base, "selected-project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"", "info", "success", "warning", "failure"} {
		args := []string{"--json", "-C", project, "--via", "apprise", "notify"}
		expected := []string{"--title", "selected-project", "--body", "body-sentinel ; $(no-shell)\nnext", "--notification-type", "info"}
		if kind != "" {
			args = append(args, "--type", kind, "--title", "custom title", "--apprise-config", "provider.yaml", "--apprise-tag", "a,b c", "--apprise-format", "markdown")
			expected[1], expected[5] = "custom title", kind
			expected = append(expected, "--config", filepath.Join(base, "provider.yaml"), "--tag", "a,b c", "--input-format", "markdown")
		}
		args = append(args, expected[3])
		code, out, errOut := run(args...)
		assertResult(t, code, out, errOut, "apprise", "accepted", "")
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(err)
		}
		actual := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("argv: %#v want %#v", actual, expected)
		}
		if _, err := os.Stat(record + ".stdin"); !os.IsNotExist(err) {
			t.Fatal("provider inherited stdin")
		}
	}
	if pos, _ := in.Seek(0, io.SeekCurrent); pos != 0 {
		t.Fatal("notify consumed stdin")
	}
}

// Dispatch uncertainty must not become success or a safe-to-retry failure.
func TestNotifyOutcomes(t *testing.T) {
	environment(t)
	provider(t)
	for _, tc := range []struct {
		exit, outcome, reason string
		code                  int
	}{
		{"1", "failed", "provider_failure", 4}, {"2", "failed", "provider_failure", 4},
		{"3", "not_sent", "no_match", 3}, {"4", "failed", "partial", 4},
		{"5", "uncertain", "timeout", 5}, {"77", "failed", "provider_failure", 4},
	} {
		t.Setenv("FIXTURE_EXIT", tc.exit)
		code, out, errOut := run("--json", "--via", "apprise", "notify", "body-sentinel")
		if code != tc.code || errOut != "" {
			t.Fatalf("exit %s: %d %s", tc.exit, code, errOut)
		}
		assertResult(t, code, out, errOut, "apprise", tc.outcome, tc.reason)
		code, out, errOut = run("--via", "apprise", "notify", "body-sentinel")
		if code != tc.code || out != "" || strings.Contains(errOut, "credential://") || strings.Contains(errOut, "body-sentinel") || strings.Contains(errOut, "raw-provider-output") {
			t.Fatalf("unsafe human error: %d %s %s", code, out, errOut)
		}
	}
	t.Setenv("PATH", t.TempDir())
	code, out, errOut := run("--json", "--via", "apprise", "notify", "body-sentinel")
	assertResult(t, code, out, errOut, "apprise", "not_sent", "missing_executable")
	for _, route := range []string{"desktop", "herdr"} {
		code, out, errOut = run("--json", "--via", route, "notify", "body-sentinel")
		assertResult(t, code, out, errOut, route, "not_sent", "unsupported_route")
	}
	for _, args := range [][]string{
		{"--json", "notify"}, {"--json", "notify", "body-sentinel", "extra"},
		{"--json", "notify", "--type", "body-sentinel", "body-sentinel"},
	} {
		code, out, errOut = run(args...)
		var result delivery.Result
		if code != 2 || json.Unmarshal([]byte(out), &result) != nil || result.Outcome != "usage" || strings.Contains(out+errOut, "body-sentinel") {
			t.Fatalf("usage: %d %s %s", code, out, errOut)
		}
	}
	code, out, errOut = run("--json", "notify", "--unknown=body-sentinel")
	assertResult(t, code, out, errOut, "", "usage", "invalid_invocation")
	code, out, errOut = run("--json", "--timeout", "body-sentinel", "notify", "body-sentinel")
	assertResult(t, code, out, errOut, "", "config", "invalid_config")
}

// Cancelling an idle input pipe must exit before EOF, without dispatching.
func TestNotifyInputCancellation(t *testing.T) {
	environment(t)
	record := provider(t)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = original; writer.Close(); reader.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, errOut bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- cli.Run(ctx, []string{"tell", "--json", "--via", "apprise", "notify", "-"}, config.BuildOpts{}, &out, &errOut)
	}()
	// Give the command time to enter its read; stdin remains open throughout.
	time.AfterFunc(50*time.Millisecond, cancel)
	select {
	case code := <-done:
		assertResult(t, code, out.String(), errOut.String(), "apprise", "config", "cancelled")
	case <-time.After(2 * time.Second):
		writer.Close()
		<-done
		t.Fatal("cancellation waited for stdin EOF")
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("cancelled input launched provider")
	}
}

// A broken result stream must never claim nothing was sent after acceptance.
func TestNotifyResultWriteFailure(t *testing.T) {
	environment(t)
	output, err := os.Open(os.DevNull) // read-only: writing reliably fails
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	var errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"tell", "--json", "notify", "literal"}, config.BuildOpts{}, output, &errOut)
	if code != 0 || !strings.HasPrefix(errOut.String(), "literal\n") || !strings.Contains(errOut.String(), "notification accepted but result output failed") {
		t.Fatalf("post-delivery result: %d %q", code, errOut.String())
	}
}

func TestNotifyPreparation(t *testing.T) {
	environment(t)
	record := provider(t)
	for _, tc := range []struct{ input, want string }{
		{"a\x1b[31mred\x1b[0m\u202eb\u202c\u2066c\u2069\n\tend\r\b\x7f\u0085", "aredbc\n\tend"},
		{"a\x1b]0;hidden\aB\u009dhidden\u009cC\x1bPprivate\x1b\\D\x1b(B\u009b2JE", "aBCDE"},
		{"safe\x1b]unterminated", "safe"},
		{"safe\x1b\x7f]0;hidden\atail", "safetail"},
		{"safe\x1b\x7f[31mred\x1b[0m", "safered"},
		{"safe\x1b\b]0;hidden\atail", "safetail"},
		{"bad\xffutf8", "bad\ufffdutf8"},
	} {
		code, out, errOut := run("notify", tc.input)
		if code != 0 || out != "" || errOut != tc.want+"\n" {
			t.Fatalf("stderr normalization: %d %q %q", code, out, errOut)
		}
		code, out, errOut = run("--json", "--via", "apprise", "notify", "--title", tc.input, tc.input)
		assertResult(t, code, out, errOut, "apprise", "accepted", "")
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(string(data), "\x00")
		if args[1] != tc.want || args[3] != tc.want {
			t.Fatalf("provider received unprepared input: %q", args)
		}
	}
	for _, count := range []int{1, 10000} {
		input := strings.Repeat("界", count)
		code, out, errOut := run("--json", "--via", "apprise", "notify", "--title", input, input)
		assertResult(t, code, out, errOut, "apprise", "accepted", "")
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(string(data), "\x00")
		for _, field := range []struct {
			text  string
			limit int
		}{{args[1], 256}, {args[3], 16384}} {
			want := input
			if len(want) > field.limit {
				want = strings.Repeat("界", (field.limit-len("[truncated]"))/3) + "[truncated]"
			}
			if field.text != want || !utf8.ValidString(field.text) || len(field.text) > field.limit {
				t.Fatalf("incorrect truncation at limit %d: %d bytes", field.limit, len(field.text))
			}
		}
	}
}

func TestNotifyPreparationRejectsBeforeDispatch(t *testing.T) {
	environment(t)
	record := provider(t)
	for _, route := range []string{"stderr", "apprise"} {
		for _, tc := range []struct{ title, body, reason string }{
			{"title\x00", "body-sentinel", "invalid_message"},
			{"title", "body-sentinel\x00", "invalid_message"},
			{"title", strings.Repeat("x", 17000) + "\x00", "invalid_message"},
			{"title", "\x1b[31m\u202e", "empty_message"},
		} {
			stdin(t, tc.body)
			code, out, errOut := run("--json", "--via", route, "notify", "--title", tc.title, "-")
			assertResult(t, code, out, errOut, route, "config", tc.reason)
			if code != 1 || errOut != "" {
				t.Fatalf("invalid input dispatched: %d %q", code, errOut)
			}
		}
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("invalid input launched provider")
	}
}

// Prevent secrets reaching argv/stderr, bypass via inline allow comments, and
// partial tokens escaping when truncation would otherwise hide their suffix.
func TestNotifySecrets(t *testing.T) {
	for _, policy := range []string{"redact", "reject"} {
		for _, field := range []string{"title", "body"} {
			t.Run(policy+"/"+field, func(t *testing.T) {
				base := environment(t)
				record := provider(t)
				if policy == "reject" {
					if err := os.WriteFile(filepath.Join(base, ".tell.yaml"), []byte("secrets: {policy: reject}\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				// Construct a synthetic GitHub token, not a usable credential.
				token := "ghp_" + "aB3dE6fG9hJ2kL5mN8pQ1rS4tU7vW0xY3zA6"
				title, body := "title", "message"
				if field == "title" {
					title = token
				} else {
					body = token
				}
				code, out, errOut := run("--json", "--via", "apprise", "notify", "--title", title, body)
				if strings.Contains(out+errOut, token) {
					t.Fatal("secret leaked in result")
				}
				if policy == "reject" {
					assertResult(t, code, out, errOut, "apprise", "config", "secret_detected")
					if code != 1 {
						t.Fatal("reject did not exit 1")
					}
					if _, err := os.Stat(record); !os.IsNotExist(err) {
						t.Fatal("reject launched provider")
					}
					return
				}
				assertResult(t, code, out, errOut, "apprise", "accepted", "")
				var result delivery.Result
				if err := json.Unmarshal([]byte(out), &result); err != nil || result.Redactions != 1 {
					t.Fatalf("redaction count: %s", out)
				}
				data, err := os.ReadFile(record)
				if err != nil || strings.Contains(string(data), token) || !strings.Contains(string(data), "[REDACTED]") {
					t.Fatal("provider did not receive redacted text")
				}
				for _, input := range []string{
					token + " " + token + " # gitleaks:allow",
					"ghp_\x1b[31m" + strings.TrimPrefix(token, "ghp_"),
					strings.Repeat("x", 16370) + " " + token,
				} {
					code, _, errOut = run("--json", "notify", input)
					if code != 0 || strings.Contains(errOut, "ghp_") || !strings.Contains(errOut, "[REDACTED]") {
						t.Fatalf("unsafe stderr preparation: exit %d", code)
					}
				}
			})
		}
	}
}

func TestNotifyInterruption(t *testing.T) {
	for _, cancelAfterStart := range []bool{false, true} {
		t.Run(map[bool]string{false: "timeout", true: "cancellation"}[cancelAfterStart], func(t *testing.T) {
			environment(t)
			record := provider(t)
			t.Setenv("FIXTURE_WAIT", "yes")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := "100ms"
			if cancelAfterStart {
				timeout = "5s"
			}
			var out, errOut bytes.Buffer
			done := make(chan int, 1)
			go func() {
				done <- cli.Run(ctx, []string{"tell", "--json", "--timeout", timeout, "--via", "apprise", "notify", "body-sentinel"}, config.BuildOpts{}, &out, &errOut)
			}()
			if cancelAfterStart {
				deadline := time.After(3 * time.Second)
				ticker := time.NewTicker(time.Millisecond)
				defer ticker.Stop()
			waitStart:
				for {
					select {
					case <-ticker.C:
						if _, err := os.Stat(record); err == nil {
							cancel()
							break waitStart
						}
					case <-deadline:
						cancel()
						<-done
						t.Fatal("provider did not start")
					}
				}
			}
			code := <-done
			reason := "timeout"
			if cancelAfterStart {
				reason = "cancelled"
			}
			assertResult(t, code, out.String(), errOut.String(), "apprise", "uncertain", reason)
		})
	}
}
