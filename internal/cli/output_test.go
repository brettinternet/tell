package cli_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/cli"
	"github.com/brettinternet/tell/internal/config"
	"github.com/creack/pty"
)

// Use a real terminal for one stream and a pipe-like buffer for the other.
func runTerminal(t *testing.T, terminalStdout bool, args ...string) (int, string, string) {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer slave.Close()
	var redirected bytes.Buffer
	type capture struct {
		data []byte
		err  error
	}
	done := make(chan capture, 1)
	go func() {
		data, err := bufio.NewReader(master).ReadBytes(0)
		done <- capture{data, err}
	}()
	var out, diag io.Writer = &redirected, slave
	if terminalStdout {
		out, diag = slave, &redirected
	}
	code := cli.Run(context.Background(), append([]string{"tell"}, args...), config.BuildOpts{Version: "test"}, out, diag)
	// Drain through a marker before closing: macOS can discard unread PTY output
	// when the last slave closes. No command under test emits a literal NUL.
	if _, err := slave.Write([]byte{0}); err != nil {
		t.Fatal(err)
	}
	var text string
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		text = strings.ReplaceAll(strings.TrimSuffix(string(result.data), "\x00"), "\r\n", "\n")
	case <-time.After(5 * time.Second):
		t.Fatal("terminal output did not drain")
	}
	if terminalStdout {
		return code, text, redirected.String()
	}
	return code, redirected.String(), text
}

// Redirecting one stream must not disable the other, and disabling styling must
// work before/after commands and even when flag parsing fails.
func TestTerminalStyleControls(t *testing.T) {
	environment(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	for _, tc := range []struct {
		name, term, noColor string
		args                []string
		plain               bool
	}{
		{name: "automatic"},
		{name: "table", args: []string{"config"}},
		{name: "flag", args: []string{"--no-color"}, plain: true},
		{name: "after command", args: []string{"config", "--no-color"}, plain: true},
		{name: "help", args: []string{"send", "--help"}},
		{name: "plain help", args: []string{"send", "--help", "--no-color"}, plain: true},
		{name: "help parse failure", args: []string{"--help", "--unknown", "--no-color"}, plain: true},
		{name: "root terminator", args: []string{"--", "doctor", "--no-color"}, plain: true},
		{name: "nested terminator", args: []string{"config", "--", "show", "--no-color"}, plain: true},
		{name: "environment", noColor: "0", plain: true},
		{name: "dumb", term: "dumb", plain: true},
		{name: "false flag", args: []string{"--no-color=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.noColor)
			if tc.term != "" {
				t.Setenv("TERM", tc.term)
			}
			code, out, diag := runTerminal(t, true, tc.args...)
			if code != 0 || diag != "" || strings.Contains(out, "\x1b[") == tc.plain {
				t.Fatalf("%d %q %q", code, out, diag)
			}
			_, piped, _ := run(tc.args...)
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(out, "")
			if plain != piped || strings.Contains(piped, "\x1b") {
				t.Fatalf("styling changed layout: terminal %q; pipe %q", plain, piped)
			}
		})
	}
	for _, tc := range []struct {
		args  []string
		plain bool
	}{
		{[]string{"--unknown"}, false},
		{[]string{"--unknown", "--no-color"}, true},
		{[]string{"--no-color", "--no-color=invalid"}, true},
		{[]string{"notify", "--no-color", "--unknown"}, true},
		// A flag value or positional message is not a presentation switch.
		{[]string{"notify", "--apprise-tag", "--no-color", "--unknown"}, false},
		{[]string{"send", "--", "--no-color"}, false},
		{[]string{"send", "--", "doctor", "--no-color"}, false},
	} {
		code, out, diag := runTerminal(t, false, tc.args...)
		if code == 0 || out != "" || strings.Contains(diag, "\x1b[") == tc.plain {
			t.Fatalf("%v: %d %q %q", tc.args, code, out, diag)
		}
	}
	// A terminal stdout cannot cause ANSI on redirected stderr, or vice versa.
	_, out, diag := runTerminal(t, true, "send")
	if out != "" || strings.Contains(diag, "\x1b") {
		t.Fatalf("redirected stderr: %q %q", out, diag)
	}
	_, out, diag = runTerminal(t, false, "config")
	if strings.Contains(out+diag, "\x1b") {
		t.Fatalf("redirected stdout: %q %q", out, diag)
	}
}

// Styling must never corrupt machine-readable streams or alter delivery bodies.
func TestTerminalProtocolAndPayloadIsolation(t *testing.T) {
	environment(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	for _, args := range [][]string{
		{"--json", "config"}, {"--unknown", "--json"}, {"--json", "send"},
		{"--json", "--json=invalid"},
		{"--", "doctor", "--json"},
		{"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"},
		{"--generate-shell-completion"}, {"--json", "--help"},
		{"notify", "--type", "success", "body-sentinel"},
		{"--json", "notify", "body-sentinel"},
	} {
		for _, terminalStdout := range []bool{false, true} {
			code, out, diag := runTerminal(t, terminalStdout, args...)
			wantCode, wantOut, wantDiag := run(args...)
			if args[0] == "--json" && args[len(args)-1] != "--help" && !json.Valid([]byte(out)) {
				t.Fatalf("invalid JSON for %v: %q", args, out)
			}
			if code != wantCode || out != wantOut || diag != wantDiag || strings.Contains(out+diag, "\x1b") {
				t.Fatalf("%v: %d %q %q; plain %d %q %q", args, code, out, diag, wantCode, wantOut, wantDiag)
			}
		}
	}
	in := stdin(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"style-test","version":"1"}}}`+"\n")
	defer in.Close()
	code, out, diag := runTerminal(t, true, "mcp")
	if code != 0 || !json.Valid([]byte(out)) || strings.Contains(out+diag, "\x1b") {
		t.Fatalf("MCP: %d %q %q", code, out, diag)
	}
}

// Optional unavailable routes must not look like selected-route failures, and
// receipt styling must not imply completion or reach a provider's message argv.
func TestTerminalReadinessAndReceipts(t *testing.T) {
	environment(t)
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("NO_COLOR", "")
	t.Setenv("PATH", t.TempDir())
	code, out, diag := runTerminal(t, true, "doctor", "--apprise-tag", "configured")
	if code != 0 || diag != "" || !strings.Contains(out, "\x1b[1mstderr (selected):\x1b[0m \x1b[32mready\x1b[0m") || !strings.Contains(out, "apprise: not ready\n  apprise was not found") || strings.Contains(out, "\x1b[31m") {
		t.Fatalf("optional route: %d %q %q", code, out, diag)
	}
	code, out, diag = runTerminal(t, true, "doctor", "--via", "apprise")
	if code != 1 || !strings.Contains(out, "\x1b[1mapprise (selected):\x1b[0m \x1b[31mnot ready\x1b[0m\n  ") {
		t.Fatalf("selected route: %d %q %q", code, out, diag)
	}
	record := sendProvider(t)
	code, out, diag = runTerminal(t, false, "send", "--send-via", "hcom", "--to", "reviewer", "body-sentinel")
	if code != 0 || out != "" || !strings.Contains(diag, "outcome=\x1b[32maccepted\x1b[0m") || strings.Contains(diag, "body-sentinel") {
		t.Fatalf("accepted receipt: %d %q %q", code, out, diag)
	}
	payload, err := os.ReadFile(record + ".sent")
	if err != nil || !strings.Contains(string(payload), "body-sentinel") || bytes.Contains(payload, []byte("\x1b")) {
		t.Fatalf("provider payload: %q %v", payload, err)
	}
	t.Setenv("FIXTURE_WAIT", "yes")
	code, out, diag = runTerminal(t, false, "send", "--send-via", "hcom", "--to", "reviewer", "--timeout", "100ms", "body-sentinel")
	if code != 5 || out != "" || !strings.Contains(diag, "outcome=\x1b[33muncertain\x1b[0m") {
		t.Fatalf("uncertain receipt: %d %q %q", code, out, diag)
	}
}
