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
