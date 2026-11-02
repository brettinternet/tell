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

// Human failures must explain the fix without losing the receipt or leaking the body.
func TestSendHumanFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"input", []string{"send"}, "send requires a message or - for stdin"},
		{"route", []string{"send", "body-sentinel"}, "send.via in personal config, --send-via, or TELL_SEND_VIA"},
		{"target", []string{"send", "--send-via", "hcom", "body-sentinel"}, "send.to in personal config, --to, or TELL_SEND_TO"},
		{"session", []string{"send", "--send-via", "herdr", "--to", "reviewer", "body-sentinel"}, "herdr.session in personal config, --herdr-session, or TELL_HERDR_SESSION"},
		{"executable", []string{"send", "--send-via", "hcom", "--to", "reviewer", "body-sentinel"}, "hcom was not found on PATH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment(t)
			t.Setenv("PATH", t.TempDir())
			code, out, diag := run(tc.args...)
			lines := strings.Split(strings.TrimSuffix(diag, "\n"), "\n")
			if code == 0 || out != "" || len(lines) != 2 || !strings.Contains(lines[0], "outcome=") || !strings.Contains(lines[1], tc.want) || strings.Contains(diag, "body-sentinel") {
				t.Fatalf("%d %q %q", code, out, diag)
			}
			code, out, diag = run(append([]string{"--json"}, tc.args...)...)
			var result delivery.Result
			if code == 0 || json.Unmarshal([]byte(out), &result) != nil || result.Failure == nil || strings.Count(diag, "\n") != 1 || strings.Contains(out+diag, "body-sentinel") {
				t.Fatalf("JSON: %d %q %q", code, out, diag)
			}
		})
	}
}

func sendProvider(t *testing.T) string {
	t.Helper()
	record := provider(t, "hcom", "herdr")
	script := `#!/bin/sh
if read -r input; then exit 92; fi
if [ "$FIXTURE_PROBE_WAIT" = yes ]; then exec /bin/sleep 30; fi
if [ "$*" = "session list --json" ]; then
 printf '%s' "$FIXTURE_SESSIONS"
 exit 0
fi
if [ "$FIXTURE_OVERSIZE" = yes ]; then
 /usr/bin/head -c 1100000 /dev/zero | /usr/bin/tr '\000' ' '
 exit 0
fi
if [ "$1" = list ]; then
 printf '%s' "$FIXTURE_AGENTS"
 exit 0
fi
if [ "$1" = --session ]; then
 [ "$2" = work ] || exit 93
 if /usr/bin/env | /usr/bin/grep '^HERDR_' >/dev/null; then exit 94; fi
 if [ "$4" = get ]; then
  printf '%s' "$FIXTURE_AGENT"
  exit 0
 fi
 if [ "$4" = list ]; then
  printf '%s' "$FIXTURE_AGENT_LIST"
  exit 0
 fi
fi
printf '%s\000' "$@" >> "$FIXTURE_RECORD.sent"
printf 'call\n' >> "$FIXTURE_RECORD.calls"
if [ "$FIXTURE_WAIT" = yes ]; then exec /bin/sleep 30; fi
printf '%s' "$FIXTURE_RESPONSE"
printf 'raw-provider-output credential://private@example.invalid\n' >&2
exit "${FIXTURE_EXIT:-0}"
`
	for _, name := range []string{"hcom", "herdr"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(record), name), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("FIXTURE_SESSIONS", `{"sessions":[{"name":"work","running":true}]}`)
	t.Setenv("FIXTURE_AGENTS", `[{"name":"reviewer","status":"listening"}]`)
	t.Setenv("FIXTURE_AGENT", `{"result":{"agent":{"agent_status":"idle","pane_id":"w1:p2"}}}`)
	t.Setenv("FIXTURE_RESPONSE", `{"result":{"type":"agent_prompted"}}`)
	t.Setenv("HERDR_SESSION", "wrong-session")
	return record
}

func sendArgs(route string) []string {
	return []string{"--json", "send", "--send-via", route, "--to", "reviewer", "--herdr-session", "work"}
}

func checkSendReceipt(t *testing.T, code int, out, diag, route, outcome, reason string) delivery.Result {
	t.Helper()
	assertResult(t, code, out, diag, route, outcome, reason)
	var result delivery.Result
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if strings.Count(diag, "\n") != 1 || !strings.Contains(diag, "outcome="+outcome) || !strings.Contains(diag, "route=") || !strings.Contains(diag, "recipient=") || !strings.Contains(diag, "intent=") {
		t.Fatalf("receipt: %q", diag)
	}
	return result
}

// Explicit targeting and -- prevent body mentions/options from broadcasting;
// literal and explicit stdin messages must cross exactly the same boundary.
func TestOutboundSendFlagsDoNotOverrideDormantListenerSelectors(t *testing.T) {
	environment(t)
	record := sendProvider(t)
	configuration := writeUserConfig(t, `send: {via: hcom, to: reviewer}
listen:
  slack: {app_token_env: TELL_TEST_SLACK_SECRET}
  routes:
    - name: inherited-send
      source: slack
      type: event
      send: {message: hi}
`)
	code, out, diag := run("--config", configuration, "--json", "send", "--send-via", "herdr", "--to", "w1:p2", "--herdr-session", "work", "hello")
	result := checkSendReceipt(t, code, out, diag, "herdr", "accepted", "")
	if result.Recipient != "w1:p2" {
		t.Fatalf("outbound recipient: %+v", result)
	}
	if _, err := os.Stat(record + ".sent"); err != nil {
		t.Fatalf("explicit Herdr send did not reach its fixture: %v", err)
	}
}

func TestSendLiteralAndStdin(t *testing.T) {
	for _, route := range []string{"hcom", "herdr"} {
		for _, input := range []string{"literal", "stdin", "words", "leading-dash", "middle-dash", "missing"} {
			for _, intent := range []string{"inform", "request"} {
				t.Run(route+intent+input, func(t *testing.T) {
					environment(t)
					record := sendProvider(t)
					body := "--leading 'quoted'  \t @someone\nbody-sentinel ; $(no-shell)\n"
					in := stdin(t, body)
					args := sendArgs(route)
					if intent == "request" {
						args = append(args, "--intent", intent)
					}
					switch input {
					case "stdin":
						args = append(args, "-")
					case "words":
						args = append(args, "Tests", "failed")
						body = "Tests failed"
					case "leading-dash":
						args = append(args, "-", "failed")
						body = "- failed"
					case "middle-dash":
						args = append(args, "Tests", "-", "failed")
						body = "Tests - failed"
					case "literal":
						args = append(args, "--", body)
					}
					code, out, diag := run(args...)
					if input != "stdin" {
						if pos, _ := in.Seek(0, io.SeekCurrent); pos != 0 {
							t.Fatal("non-stdin invocation consumed stdin")
						}
					}
					if input == "missing" {
						checkSendReceipt(t, code, out, diag, route, "usage", "invalid_input")
						if code != 2 {
							t.Fatalf("missing message exit: %d", code)
						}
						if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
							t.Fatal("missing message dispatched")
						}
						return
					}
					result := checkSendReceipt(t, code, out, diag, route, "accepted", "")
					if result.Recipient != "reviewer" || result.Intent != intent {
						t.Fatalf("metadata: %s", out)
					}
					payload := "[Tell automation: tell-automation; route: " + route + "; intent: " + intent + "]\n" + body
					want := []string{"send", "@reviewer", "--from", "tell-automation", "--intent", intent, "--json", "--", payload}
					if route == "herdr" {
						want = []string{"--session", "work", "agent", "prompt", "w1:p2", payload}
					}
					data, err := os.ReadFile(record + ".sent")
					got := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("argv: %#v want %#v: %v", got, want, err)
					}
				})
			}
		}
	}
}

// Never type into an active/blocked/unknown pane, or expand target selectors.
func TestSendRefusesUnsafeTargets(t *testing.T) {
	for _, route := range []string{"hcom", "herdr"} {
		for _, target := range []string{"", "@reviewer", "api-", "one two", "one,two", "*", "--help", "self"} {
			t.Run(route+target, func(t *testing.T) {
				environment(t)
				record := sendProvider(t)
				code, out, diag := run("--json", "send", "--send-via", route, "--to", target, "--herdr-session", "work", "body-sentinel")
				reason := "invalid_target"
				if target == "" {
					reason = "missing_target"
				}
				checkSendReceipt(t, code, out, diag, route, "not_sent", reason)
				if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
					t.Fatal("sent unsafe target")
				}
			})
		}
	}
	// Unknown or nonparticipating hcom states must not appear ready or send.
	for _, state := range []string{"unknown", "error", "launching", "inactive", ""} {
		t.Run("hcom-"+state, func(t *testing.T) {
			environment(t)
			record := sendProvider(t)
			t.Setenv("FIXTURE_AGENTS", `[{"name":"reviewer","status":"`+state+`"}]`)
			code, out, diag := run(append(sendArgs("hcom"), "body-sentinel")...)
			checkSendReceipt(t, code, out, diag, "hcom", "not_sent", "missing_target")
			code, out, diag = run("--json", "doctor", "--send-via", "hcom", "--to", "reviewer")
			var report struct {
				Ready  bool
				Routes []delivery.Readiness
			}
			if code != 1 || json.Unmarshal([]byte(out), &report) != nil || report.Ready || diag != "" {
				t.Fatalf("unsafe readiness: %d %s %s", code, out, diag)
			}
			if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
				t.Fatal("sent to unconfirmed participant")
			}
		})
	}
	for _, state := range []string{"working", "blocked", "unknown", ""} {
		t.Run(state, func(t *testing.T) {
			environment(t)
			record := sendProvider(t)
			t.Setenv("FIXTURE_AGENT", `{"result":{"agent":{"pane_id":"w1:p2","agent_status":"`+state+`"}}}`)
			code, out, diag := run(append(sendArgs("herdr"), "body-sentinel")...)
			reason := "busy"
			if state == "blocked" {
				reason = "blocked"
			}
			checkSendReceipt(t, code, out, diag, "herdr", "not_sent", reason)
			if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
				t.Fatal("prompted unsafe state")
			}
		})
	}
}

// Preserve dispatch uncertainty, body-free receipts, and exactly-once attempts.
func TestSendOutcomes(t *testing.T) {
	for _, route := range []string{"hcom", "herdr"} {
		for _, mode := range []string{"missing-executable", "missing-target", "provider-failure", "timeout", "cancelled", "probe-timeout", "blocked-race"} {
			if mode == "blocked-race" && route != "herdr" {
				continue
			}
			t.Run(route+mode, func(t *testing.T) {
				environment(t)
				record := sendProvider(t)
				outcome, reason := "not_sent", "missing_target"
				args := append(sendArgs(route), "body-sentinel")
				ctx := context.Background()
				switch mode {
				case "missing-executable":
					t.Setenv("PATH", t.TempDir())
					reason = "missing_executable"
				case "missing-target":
					t.Setenv("FIXTURE_AGENTS", `[]`)
					t.Setenv("FIXTURE_AGENT", `{"error":{"code":"agent_not_found"}}`)
				case "provider-failure":
					t.Setenv("FIXTURE_EXIT", "1")
					outcome, reason = "failed", "provider_failure"
				case "blocked-race":
					t.Setenv("FIXTURE_RESPONSE", `{"error":{"code":"agent_blocked"}}`)
					t.Setenv("FIXTURE_EXIT", "1")
					reason = "blocked"
				case "probe-timeout":
					// A stalled read-only check sent nothing; uncertain would forbid a safe resend.
					t.Setenv("FIXTURE_PROBE_WAIT", "yes")
					args = append([]string{"--timeout", "150ms"}, args...)
					reason = "timeout"
				case "timeout", "cancelled":
					t.Setenv("FIXTURE_WAIT", "yes")
					outcome, reason = "uncertain", mode
					if mode == "timeout" {
						// Leave room for Herdr's probes under -race so the deadline hits dispatch.
						args = append([]string{"--timeout", "1s"}, args...)
					} else {
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						defer cancel()
						// Cancel only after the fixture records dispatch, not during preparation.
						done := make(chan struct{})
						defer close(done)
						go func() {
							ticker := time.NewTicker(time.Millisecond)
							defer ticker.Stop()
							for {
								select {
								case <-done:
									return
								case <-ticker.C:
									if _, err := os.Stat(record + ".calls"); err == nil {
										cancel()
										return
									}
								}
							}
						}()
					}
				}
				var out, diag bytes.Buffer
				code := cli.Run(ctx, append([]string{"tell"}, args...), config.BuildOpts{}, &out, &diag)
				checkSendReceipt(t, code, out.String(), diag.String(), route, outcome, reason)
				calls, _ := os.ReadFile(record + ".calls")
				if strings.Count(string(calls), "call") > 1 {
					t.Fatal("retried delivery")
				}
			})
		}
	}
}

// Doctor must use read-only probes; normalization/redaction must also apply to send.
func TestSendReadinessAndPreparation(t *testing.T) {
	for _, route := range []string{"hcom", "herdr"} {
		t.Run(route, func(t *testing.T) {
			environment(t)
			record := sendProvider(t)
			cfg := "send: {via: " + route + ", to: reviewer, from: nightly}\nherdr: {session: work}\n"
			writeUserConfig(t, cfg)
			code, out, diag := run("--json", "doctor")
			var report struct {
				Ready  bool
				Routes []delivery.Readiness
			}
			if code != 0 || json.Unmarshal([]byte(out), &report) != nil || !report.Ready || diag != "" {
				t.Fatalf("doctor: %d %s %s", code, out, diag)
			}
			found := false
			for _, r := range report.Routes {
				if r.Route == "send/"+route && r.Ready {
					found = true
				}
			}
			if !found {
				t.Fatalf("no send readiness: %s", out)
			}
			// The fixture records unexpected actions, including sends.
			if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
				t.Fatal("doctor sent")
			}
			token := "gh" + "p_" + strings.Repeat("A1b2C3d4", 5)
			code, out, diag = run("--json", "send", "\x1b[31msecret="+token+"\x1b[0m")
			result := checkSendReceipt(t, code, out, diag, route, "accepted", "")
			sent, _ := os.ReadFile(record + ".sent")
			if result.Redactions == 0 || strings.Contains(string(sent), token) || strings.Contains(string(sent), "\x1b") || !strings.Contains(string(sent), "Tell automation: nightly") {
				t.Fatalf("preparation: %s", out)
			}
		})
	}
}

// targets must list only values send accepts, never prompt or send, and never
// pass titles, session paths, or other provider fields through.
func TestTargetsReadOnly(t *testing.T) {
	private := `"title":"private-title","terminal_title":"private-title","cwd":"/private/path","agent_session":{"kind":"path","value":"/private/session.jsonl"}`
	for _, tc := range []struct {
		route, fixture, list string
		want                 []delivery.Target
	}{
		{"herdr", "FIXTURE_AGENT_LIST", `{"result":{"agents":[` +
			`{"name":"reviewer","agent":"claude","agent_status":"idle","pane_id":"w1:p1",` + private + `},` +
			`{"agent":"pi","agent_status":"working","pane_id":"w1:p2",` + private + `},` +
			`{"name":"bad name","agent":"Private Kind","agent_status":"blocked","pane_id":"w1:p3"},` +
			`{"agent":"codex","agent_status":"private-state","pane_id":"w1:p4"},` +
			`{"agent":"codex","agent_status":"done","pane_id":"/private/pane"}]}}`,
			[]delivery.Target{{To: "reviewer", Kind: "claude", Status: "idle", Ready: true}, {To: "w1:p2", Kind: "pi", Status: "working"}, {To: "w1:p3", Kind: "unknown", Status: "blocked"}, {To: "w1:p4", Kind: "codex", Status: "unknown"}}},
		{"hcom", "FIXTURE_AGENTS", `[{"name":"reviewer","status":"listening","tool":"claude","directory":"/private/path"},` +
			`{"name":"twin","status":"active","tool":"codex"},{"name":"twin","status":"blocked","tool":"codex"},` +
			`{"name":"gone","status":"inactive","tool":"claude"},{"name":"@private","status":"active"}]`,
			[]delivery.Target{{To: "reviewer", Kind: "claude", Status: "listening", Ready: true}, {To: "twin", Kind: "codex", Status: "duplicate"}}},
		{"herdr", "FIXTURE_AGENT_LIST", `{"result":{"agents":[]}}`, []delivery.Target{}},
	} {
		t.Run(tc.route, func(t *testing.T) {
			environment(t)
			record := sendProvider(t)
			t.Setenv(tc.fixture, tc.list)
			args := []string{"targets", "--send-via", tc.route, "--herdr-session", "work"}
			code, out, diag := run(append([]string{"--json"}, args...)...)
			decoder := json.NewDecoder(strings.NewReader(out))
			var report struct {
				SchemaVersion int               `json:"schema_version"`
				Route         string            `json:"route"`
				Targets       []delivery.Target `json:"targets"`
			}
			if code != 0 || diag != "" || decoder.Decode(&report) != nil || decoder.More() || report.SchemaVersion != 1 || report.Route != tc.route || !reflect.DeepEqual(report.Targets, tc.want) {
				t.Fatalf("%d %q %q", code, out, diag)
			}
			humanCode, human, humanDiag := run(args...)
			if humanCode != 0 || humanDiag != "" || (len(tc.want) == 0) != strings.HasPrefix(human, "No ") {
				t.Fatalf("human: %d %q %q", humanCode, human, humanDiag)
			}
			if strings.Contains(out+human, "private") || strings.Contains(out+human, "raw-provider-output") {
				t.Fatalf("leaked provider fields: %s %s", out, human)
			}
			if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
				t.Fatal("targets sent or prompted")
			}
		})
	}
	for _, tc := range []struct {
		name, route, outcome, reason string
		setup                        func(t *testing.T)
	}{
		{"missing-executable", "herdr", "not_sent", "missing_executable", func(t *testing.T) { t.Setenv("PATH", t.TempDir()) }},
		{"missing-executable", "hcom", "not_sent", "missing_executable", func(t *testing.T) { t.Setenv("PATH", t.TempDir()) }},
		{"stopped-session", "herdr", "not_sent", "session_not_running", func(t *testing.T) {
			t.Setenv("FIXTURE_SESSIONS", `{"sessions":[{"name":"work","running":false}]}`)
		}},
		{"timeout", "herdr", "not_sent", "timeout", func(t *testing.T) { t.Setenv("FIXTURE_PROBE_WAIT", "yes") }},
		{"timeout", "hcom", "not_sent", "timeout", func(t *testing.T) { t.Setenv("FIXTURE_PROBE_WAIT", "yes") }},
		{"malformed", "herdr", "not_sent", "invalid_response", func(t *testing.T) { t.Setenv("FIXTURE_AGENT_LIST", `{"result":{}}`) }},
		{"malformed", "hcom", "not_sent", "invalid_response", func(t *testing.T) { t.Setenv("FIXTURE_AGENTS", `{"private":true}`) }},
		{"oversized", "herdr", "not_sent", "invalid_response", func(t *testing.T) { t.Setenv("FIXTURE_OVERSIZE", "yes") }},
		{"oversized", "hcom", "not_sent", "invalid_response", func(t *testing.T) { t.Setenv("FIXTURE_OVERSIZE", "yes") }},
	} {
		t.Run(tc.route+"-"+tc.name, func(t *testing.T) {
			environment(t)
			record := sendProvider(t)
			tc.setup(t)
			code, out, diag := run("--json", "--timeout", "150ms", "targets", "--send-via", tc.route, "--herdr-session", "work")
			assertResult(t, code, out, diag, tc.route, tc.outcome, tc.reason)
			if diag != "" {
				t.Fatalf("diagnostics: %q", diag)
			}
			if _, err := os.Stat(record + ".sent"); !os.IsNotExist(err) {
				t.Fatal("targets sent or prompted")
			}
		})
	}
}
