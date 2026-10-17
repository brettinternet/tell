package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/delivery"
)

// A readiness probe must not turn into delivery, leak provider secrets, or fail
// the selected route just because another configured route is unavailable.
func TestDoctorReadOnly(t *testing.T) {
	environment(t)
	record := provider(t, "herdr", "osascript", "notify-send", "personal")
	dir := filepath.Dir(record)
	apprise := `#!/bin/sh
printf '%s\000' "$@" > "$FIXTURE_RECORD.apprise"
[ "$1" = --dry-run ] || exit 91
if read -r input; then exit 92; fi
printf '   1. credential://private@example.invalid\n       uid: private-id\n   2. credential://second@example.invalid\n'
printf 'private-provider-error' >&2
exit "${FIXTURE_EXIT:-0}"
`
	herdr := `#!/bin/sh
printf '%s\000' "$@" > "$FIXTURE_RECORD.herdr"
[ "$*" = 'session list --json' ] || exit 91
if /usr/bin/env | /usr/bin/grep '^HERDR_' >/dev/null; then exit 92; fi
printf '%s' "$FIXTURE_SESSIONS"
`
	for name, script := range map[string]string{"apprise": apprise, "herdr": herdr} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HERDR_SESSION", "wrong-session")
	t.Setenv("HERDR_SOCKET_PATH", "private-socket")
	t.Setenv("FIXTURE_SESSIONS", `{"sessions":[{"name":"private-session","running":true,"socket_path":"private-path"}]}`)
	cfg := "notify: {via: apprise, command: [" + filepath.Join(dir, "personal") + ", private-argument]}\napprise: {config: private-config, tag: private-tag, format: markdown}\nherdr: {session: private-session}\n"
	writeUserConfig(t, cfg)
	for _, route := range []string{"apprise", "herdr", "command", "desktop", "stderr"} {
		code, out, diagnostics := run("--json", "doctor", "--via", route)
		var report struct {
			SchemaVersion int                  `json:"schema_version"`
			Selected      string               `json:"selected"`
			Ready         bool                 `json:"ready"`
			Routes        []delivery.Readiness `json:"routes"`
		}
		if json.Unmarshal([]byte(out), &report) != nil || code != 0 || !report.Ready || report.SchemaVersion != 1 || report.Selected != route || diagnostics != "" {
			t.Fatalf("%s: %d %s %s", route, code, out, diagnostics)
		}
		found := map[string]delivery.Readiness{}
		for _, result := range report.Routes {
			found[result.Route] = result
		}
		if len(found) != len(report.Routes) || len(found) < 3 || found["apprise"].Services == nil || *found["apprise"].Services != 2 {
			t.Fatalf("routes: %s", out)
		}
		for _, args := range [][]string{{"doctor", "--via", route}, {"--json", "doctor", "--via", route}} {
			code, out, diagnostics = run(args...)
			if code != 0 {
				t.Fatalf("%d %s %s", code, out, diagnostics)
			}
			for _, secret := range []string{"private-", "credential://", "second@example"} {
				if strings.Contains(out+diagnostics, secret) {
					t.Fatalf("leaked provider data: %s %s", out, diagnostics)
				}
			}
		}
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("executed desktop or personal command")
	}
	got, err := os.ReadFile(record + ".apprise")
	want := strings.Join([]string{"--dry-run", "--config", filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "tell", "private-config"), "--tag", "private-tag", "--input-format", "markdown", ""}, "\x00")
	if err != nil || string(got) != want {
		t.Fatalf("dry run argv: %q %v", got, err)
	}
	got, err = os.ReadFile(record + ".herdr")
	if err != nil || string(got) != "session\x00list\x00--json\x00" {
		t.Fatalf("session argv: %q %v", got, err)
	}
}

// Failed probes and empty/malformed lists must never be reported as ready.
func TestDoctorFailures(t *testing.T) {
	for _, tc := range []struct{ name, route, body, exit, reason string }{
		{"no services", "apprise", "", "0", "no_services"},
		{"no match exit", "apprise", "", "3", "no_services"},
		{"provider failure", "apprise", "   1. private-url\n", "1", "probe_failed"},
		{"stopped session", "herdr", `{"sessions":[{"name":"target","running":false}]}`, "0", "session_not_running"},
		{"different session", "herdr", `{"sessions":[{"name":"other","running":true}]}`, "0", "session_not_running"},
		{"malformed", "herdr", "private-invalid-json", "0", "invalid_response"},
		{"missing list", "herdr", `{}`, "0", "invalid_response"},
		{"session failure", "herdr", "private-error", "1", "probe_failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment(t)
			record := provider(t, "herdr")
			script := "#!/bin/sh\nprintf '%s' \"$FIXTURE_OUTPUT\"\nprintf 'private-error' >&2\nexit \"$FIXTURE_EXIT\"\n"
			if err := os.WriteFile(filepath.Join(filepath.Dir(record), tc.route), []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("FIXTURE_OUTPUT", tc.body)
			t.Setenv("FIXTURE_EXIT", tc.exit)
			for _, selected := range []string{tc.route, "stderr"} {
				code, out, diagnostics := run("--json", "doctor", "--via", selected, "--apprise-tag", "private-tag", "--herdr-session", "target")
				want := 1
				if selected == "stderr" {
					want = 0
				}
				var report struct {
					Routes []delivery.Readiness `json:"routes"`
				}
				if code != want || json.Unmarshal([]byte(out), &report) != nil || strings.Contains(out+diagnostics, "private-") {
					t.Fatalf("%d %s %s", code, out, diagnostics)
				}
				found := false
				for _, route := range report.Routes {
					if route.Route == tc.route {
						found = true
						if route.Ready || route.Reason != tc.reason {
							t.Fatalf("%s", out)
						}
					}
				}
				if !found {
					t.Fatalf("missing route: %s", out)
				}
			}
		})
	}
}

// A noisy provider must fail closed rather than exhaust memory or accept a
// truncated response whose prefix happens to contain valid destinations.
func TestDoctorOversizedOutput(t *testing.T) {
	environment(t)
	record := provider(t)
	data := filepath.Join(filepath.Dir(record), "output")
	if err := os.WriteFile(data, []byte("   1. private-url\n"+strings.Repeat("x", 1024*1024)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FIXTURE_OUTPUT_FILE", data)
	if err := os.WriteFile(filepath.Join(filepath.Dir(record), "apprise"), []byte("#!/bin/sh\n/bin/cat \"$FIXTURE_OUTPUT_FILE\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	code, out, diagnostics := run("--json", "doctor", "--via", "apprise")
	if code != 1 || !strings.Contains(out, `"reason":"invalid_response"`) || strings.Contains(out+diagnostics, "private-url") {
		t.Fatalf("%d %s %s", code, out, diagnostics)
	}
}

func TestDoctorUnavailableAndTimeout(t *testing.T) {
	base := environment(t)
	t.Setenv("PATH", t.TempDir())
	for _, tc := range []struct{ route, reason string }{{"apprise", "missing_executable"}, {"herdr", "missing_target"}, {"desktop", "missing_executable"}, {"command", "missing_command"}} {
		code, out, diagnostics := run("--json", "doctor", "--via", tc.route)
		if code != 1 || !strings.Contains(out, `"reason":"`+tc.reason+`"`) || diagnostics != "" {
			t.Fatalf("%d %s %s", code, out, diagnostics)
		}
	}
	path := filepath.Join(base, "private-command")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeUserConfig(t, "notify: {via: command, command: ["+path+"]}\n")
	code, out, diagnostics := run("--json", "doctor")
	if code != 1 || !strings.Contains(out, "command_not_executable") || strings.Contains(out+diagnostics, path) {
		t.Fatalf("%d %s %s", code, out, diagnostics)
	}
	provider(t)
	t.Setenv("FIXTURE_WAIT", "yes")
	code, out, diagnostics = run("--json", "doctor", "--via", "apprise", "--timeout", "10ms")
	if code != 1 || !strings.Contains(out, `"reason":"timeout"`) || diagnostics != "" {
		t.Fatalf("%d %s %s", code, out, diagnostics)
	}
}
