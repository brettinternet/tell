package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHerdrNotify(t *testing.T) {
	base := environment(t)
	record := provider(t, "herdr")
	// Refuse inherited context rather than merely recording the explicit flag.
	path := filepath.Join(filepath.Dir(record), "herdr")
	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	script = []byte(strings.Replace(string(script), "#!/bin/sh\n", `#!/bin/sh
if /usr/bin/env | /usr/bin/grep '^HERDR_' >/dev/null; then
  exit 91
fi
`, 1))
	if err := os.WriteFile(path, script, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"HERDR_ENV", "HERDR_SOCKET_PATH", "HERDR_SESSION", "HERDR_PANE_ID", "HERDR_CONFIG_PATH", "HERDR_REMOTE_PREFERENCES_IDENTITY"} {
		t.Setenv(name, "wrong-destination")
	}
	code, out, errOut := run("--json", "notify", "--via", "herdr", "body-sentinel")
	assertResult(t, code, out, errOut, "herdr", "not_sent", "missing_target")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("missing session launched provider")
	}
	if err := os.WriteFile(filepath.Join(base, ".tell.yaml"), []byte("notify: {via: herdr}\nherdr: {session: selected}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{}, {"--json", "config"}, {"--json", "config", "check"}} {
		code, out, errOut = run(args...)
		if code != 0 {
			t.Fatalf("inspection: %d %s %s", code, out, errOut)
		}
		if _, err := os.Stat(record); !os.IsNotExist(err) {
			t.Fatal("inspection launched Herdr")
		}
	}
	code, out, errOut = run("--json", "notify", "--via", "herdr", "--title", "title ; $(no-shell)\x1b[31m", "body-sentinel ; $(no-shell)\nnext")
	assertResult(t, code, out, errOut, "herdr", "accepted", "")
	got, err := os.ReadFile(record)
	want := strings.Join([]string{"--session", "selected", "notification", "show", "title ; $(no-shell)", "--body", "body-sentinel ; $(no-shell)\nnext", "--sound", "none", ""}, "\x00")
	if err != nil || string(got) != want {
		t.Fatalf("argv: %q want %q (%v)", got, want, err)
	}
	if _, err := os.Stat(record + ".stdin"); !os.IsNotExist(err) {
		t.Fatal("Herdr inherited stdin")
	}
	for _, exit := range []string{"1", "2", "3", "5"} {
		t.Setenv("FIXTURE_EXIT", exit)
		code, out, errOut = run("--json", "notify", "body-sentinel")
		assertResult(t, code, out, errOut, "herdr", "failed", "provider_failure")
		code, out, errOut = run("notify", "body-sentinel")
		if code != 4 || out != "" || !strings.Contains(errOut, "configured session") || strings.Contains(errOut, "body-sentinel") || strings.Contains(errOut, "raw-provider-output") {
			t.Fatalf("unsafe failure: %d %q %q", code, out, errOut)
		}
	}
	t.Setenv("PATH", t.TempDir())
	code, out, errOut = run("--json", "notify", "body-sentinel")
	assertResult(t, code, out, errOut, "herdr", "not_sent", "missing_executable")
}

// Herdr's global parser otherwise consumes payload options, allowing a body
// such as --session=other to retarget delivery, or a --help title to exit 0.
func TestHerdrReservedPayloads(t *testing.T) {
	environment(t)
	record := provider(t, "herdr")
	for _, value := range []string{"--session=other", "--session", "--remote=host", "--remote-keybindings=server", "--handoff", "--help", "-h", "help"} {
		fields := []string{"title", "body"}
		if value == "--help" || value == "-h" || value == "help" {
			fields = []string{"title"}
		}
		for _, field := range fields {
			title, body := "title", "body-sentinel"
			if field == "title" {
				title = value
			} else {
				body = value
			}
			code, out, errOut := run("--json", "notify", "--via", "herdr", "--herdr-session", "selected", "--title", title, "--", body)
			assertResult(t, code, out, errOut, "herdr", "not_sent", "unsupported_message")
		}
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("reserved payload launched provider")
	}
}
