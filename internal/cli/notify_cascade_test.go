package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/delivery"
)

// A not_sent result may advance, but failed remains a stop by default and
// uncertain must never advance, preventing duplicate notifications.
func TestNotifyCascadeOutcomeRules(t *testing.T) {
	for _, tc := range []struct {
		name, phoneExit, chatExit, fallback string
		wantCode                            int
		wantOutcome, wantReason             string
		wantAttempts                        int
	}{
		{"not sent advances", "3", "0", "", 0, "accepted", "", 2},
		{"all steps not sent", "3", "3", "", 3, "not_sent", "no_match", 2},
		{"accepted stops", "0", "0", "", 0, "accepted", "", 1},
		{"failed stops by default", "1", "0", "", 4, "failed", "provider_failure", 1},
		{"failed opt in advances", "1", "0", "[not_sent, failed]", 0, "accepted", "", 2},
		{"uncertain never advances", "5", "0", "[not_sent, failed]", 5, "uncertain", "timeout", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := environment(t)
			record := provider(t)
			installCascadeProvider(t, record)
			t.Setenv("PHONE_EXIT", tc.phoneExit)
			t.Setenv("CHAT_EXIT", tc.chatExit)
			fallback := ""
			if tc.fallback != "" {
				fallback = "\n  fallback_on: " + tc.fallback
			}
			writeUserConfig(t, "notify:\n  via:\n    - apprise: {tag: phone}\n    - apprise: {tag: chat}"+fallback+"\n")
			code, output, diagnostics := run("--json", "notify", "body-sentinel")
			var result delivery.Result
			if json.Unmarshal([]byte(output), &result) != nil || code != tc.wantCode || result.Outcome != tc.wantOutcome || result.ExitCode() != tc.wantCode || len(result.Attempts) != tc.wantAttempts {
				t.Fatalf("cascade result: code=%d result=%s diagnostics=%s", code, output, diagnostics)
			}
			if tc.wantReason != "" && (result.Failure == nil || result.Failure.Reason != tc.wantReason) {
				t.Fatalf("final failure: %+v", result)
			}
			if len(result.Attempts) > 0 && result.Attempts[0].Label != "apprise:phone" {
				t.Fatalf("unsafe or missing label: %+v", result.Attempts)
			}
			if strings.Contains(output+diagnostics, "body-sentinel") || strings.Contains(output+diagnostics, "credential://") {
				t.Fatalf("cascade leaked message or inherited URL: %s %s", output, diagnostics)
			}
			_, phoneErr := os.Stat(record + ".phone.args")
			_, chatErr := os.Stat(record + ".chat.args")
			if phoneErr != nil || (tc.wantAttempts == 2) != (chatErr == nil) {
				t.Fatalf("unexpected provider calls: phone=%v chat=%v", phoneErr, chatErr)
			}
			if chatErr == nil {
				phoneArgs, _ := os.ReadFile(record + ".phone.argv")
				chatArgs, _ := os.ReadFile(record + ".chat.argv")
				if !reflect.DeepEqual(strings.Split(strings.TrimSuffix(string(phoneArgs), "\x00"), "\x00"), []string{"--title", filepath.Base(base), "--body", "body-sentinel", "--notification-type", "info", "--tag", "phone"}) || !strings.Contains(string(chatArgs), "--tag\x00chat\x00") {
					t.Fatalf("provider argv: phone=%q chat=%q", phoneArgs, chatArgs)
				}
				for _, tag := range []string{"phone", "chat"} {
					env, err := os.ReadFile(record + "." + tag + ".env")
					if err != nil || string(env) != "unset" {
						t.Fatalf("tagged Apprise inherited APPRISE_URLS: %q %v", env, err)
					}
				}
			}
		})
	}
}

// A via environment override chooses one route and bypasses the configured list.
func TestNotifyViaOverrideDisablesCascade(t *testing.T) {
	environment(t)
	record := provider(t)
	installCascadeProvider(t, record)
	t.Setenv("PHONE_EXIT", "3")
	writeUserConfig(t, "notify:\n  via:\n    - apprise: {tag: phone}\n    - stderr\n")
	t.Setenv("TELL_NOTIFY_VIA", "stderr")
	code, output, diagnostics := run("--json", "notify", "override-body")
	var result delivery.Result
	if json.Unmarshal([]byte(output), &result) != nil || code != 0 || result.Route != "stderr" || len(result.Attempts) != 1 || result.Attempts[0].Route != "stderr" || diagnostics != "override-body\n" {
		t.Fatalf("via override did not select one route: %d %s %q", code, output, diagnostics)
	}
	if _, err := os.Stat(record + ".phone.args"); !os.IsNotExist(err) {
		t.Fatalf("via override ran cascade route: %v", err)
	}
}

// Missing provider capability is an explicit safe outcome and can advance.
func TestNotifyCascadeUnavailableStepsAdvance(t *testing.T) {
	for _, tc := range []struct {
		name, via, reason string
		missingExecutable bool
	}{
		{"missing config", "- {apprise: {config: absent.yaml}}", "missing_config", false},
		{"missing Herdr target", "- herdr", "missing_target", false},
		{"missing Apprise executable", "- apprise", "missing_executable", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			environment(t)
			if tc.missingExecutable {
				t.Setenv("PATH", t.TempDir())
			}
			writeUserConfig(t, "notify:\n  via:\n    "+tc.via+"\n    - stderr\n")
			code, output, diagnostics := run("--json", "notify", "fallback-body")
			var result delivery.Result
			if json.Unmarshal([]byte(output), &result) != nil || code != 0 || result.Outcome != "accepted" || len(result.Attempts) != 2 || result.Attempts[0].Reason != tc.reason || result.Attempts[1].Route != "stderr" || diagnostics != "fallback-body\n" {
				t.Fatalf("unavailable-step fallback: %d %s %q", code, output, diagnostics)
			}
		})
	}
}

// The optional title notice is based on the first route's safe label/reason;
// URLs remain environment-only and are never forwarded as argv or output.
func TestNotifyCascadeNoticeAndURLsEnvironment(t *testing.T) {
	environment(t)
	record := provider(t)
	installCascadeProvider(t, record)
	t.Setenv("PHONE_EXIT", "3")
	t.Setenv("CHAT_EXIT", "0")
	writeUserConfig(t, "notify:\n  via:\n    - apprise: {tag: phone}\n    - apprise: {tag: chat}\n  fallback_notice: title\n")
	code, output, diagnostics := run("--json", "notify", "--title", "Build finished", "body-sentinel")
	if code != 0 || diagnostics != "" || strings.Contains(output, "body-sentinel") {
		t.Fatalf("notice result: %d %s %s", code, output, diagnostics)
	}
	args, err := os.ReadFile(record + ".chat.argv")
	if err != nil || !strings.Contains(string(args), "Fallback from apprise:phone (no_match): Build finished") || !strings.Contains(string(args), "--body\x00body-sentinel\x00") {
		t.Fatalf("fallback title not prepared into delivery args: %q %v", args, err)
	}

	// A urls_env step overrides inherited APPRISE_URLS, and passes no URL in argv.
	t.Setenv("PROVIDER_DESTINATIONS", "https://user:private-url-secret@example.invalid")
	urlsRecord := filepath.Join(t.TempDir(), "urls")
	t.Setenv("FIXTURE_RECORD", urlsRecord)
	writeExecutable(t, filepath.Join(filepath.Dir(record), "apprise"), `#!/bin/sh
printf '%s\000' "$@" > "$FIXTURE_RECORD.args"
printf '%s' "${APPRISE_URLS-unset}" > "$FIXTURE_RECORD.env"
if [ "$1" = --dry-run ]; then printf '   1. credential://private-url-secret@example.invalid\n'; fi
exit 0
`)
	writeUserConfig(t, "notify:\n  via:\n    - apprise: {urls_env: PROVIDER_DESTINATIONS}\n")
	code, output, diagnostics = run("--json", "doctor")
	if code != 0 || diagnostics != "" || strings.Contains(output, "private-url-secret") || !strings.Contains(output, `"services":1`) {
		t.Fatalf("urls_env doctor leaked or failed: %d %s %s", code, output, diagnostics)
	}
	code, output, diagnostics = run("--json", "notify", "body-sentinel")
	if code != 0 || diagnostics != "" || strings.Contains(output+diagnostics, "private-url-secret") {
		t.Fatalf("urls_env delivery leaked data: %d %s %s", code, output, diagnostics)
	}
	gotEnv, err := os.ReadFile(urlsRecord + ".env")
	if err != nil || string(gotEnv) != os.Getenv("PROVIDER_DESTINATIONS") {
		t.Fatalf("child APPRISE_URLS: %q %v", gotEnv, err)
	}
	gotArgs, err := os.ReadFile(urlsRecord + ".args")
	if err != nil || strings.Contains(string(gotArgs), "private-url-secret") || strings.Contains(string(gotArgs), "--config") || strings.Contains(string(gotArgs), "--tag") {
		t.Fatalf("urls_env argv leaked URL or conflicting options: %q %v", gotArgs, err)
	}

	// Missing and empty names are safe no-send outcomes and continue to stderr.
	for _, unset := range []bool{false, true} {
		if unset {
			_ = os.Unsetenv("PROVIDER_DESTINATIONS")
		} else {
			t.Setenv("PROVIDER_DESTINATIONS", "")
		}
		if err := os.Remove(urlsRecord + ".args"); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		writeUserConfig(t, "notify:\n  via:\n    - apprise: {urls_env: PROVIDER_DESTINATIONS}\n    - stderr\n")
		code, output, diagnostics = run("--json", "doctor")
		var report struct {
			Ready bool                 `json:"ready"`
			Steps []delivery.Readiness `json:"steps"`
		}
		if json.Unmarshal([]byte(output), &report) != nil || code != 0 || !report.Ready || len(report.Steps) != 2 || report.Steps[0].Reason != "missing_urls" || !report.Steps[1].Ready || diagnostics != "" {
			t.Fatalf("doctor missing urls: %d %s %s", code, output, diagnostics)
		}
		if _, err := os.Stat(urlsRecord + ".args"); !os.IsNotExist(err) {
			t.Fatalf("doctor probed empty urls step: %v", err)
		}
		code, output, diagnostics = run("--json", "notify", "fallback-body")
		var result delivery.Result
		if json.Unmarshal([]byte(output), &result) != nil || code != 0 || result.Outcome != "accepted" || len(result.Attempts) != 2 || result.Attempts[0].Reason != "missing_urls" || diagnostics != "fallback-body\n" {
			t.Fatalf("missing urls fallback: %d %s %q", code, output, diagnostics)
		}
		if _, err := os.Stat(urlsRecord + ".args"); !os.IsNotExist(err) {
			t.Fatalf("missing URLs started Apprise: %v", err)
		}
	}
}

// Invalid cascade policy is rejected during config loading, before provider work.
func TestNotifyCascadeInvalidConfigDoesNotDeliver(t *testing.T) {
	environment(t)
	record := provider(t)
	writeUserConfig(t, "notify:\n  via: [apprise, stderr]\n  fallback_on: [uncertain]\n")
	code, output, diagnostics := run("--json", "notify", "body-sentinel")
	assertResult(t, code, output, diagnostics, "", "config", "invalid_config")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("invalid cascade launched provider")
	}
}

// Human failures report reasons for every attempted step; success remains silent
// about skipped providers.
func TestNotifyCascadeHumanAttemptSummary(t *testing.T) {
	environment(t)
	record := provider(t)
	installCascadeProvider(t, record)
	t.Setenv("PHONE_EXIT", "1")
	t.Setenv("CHAT_EXIT", "1")
	writeUserConfig(t, "notify:\n  via:\n    - apprise: {tag: phone}\n    - apprise: {tag: chat}\n  fallback_on: [not_sent, failed]\n")
	code, output, diagnostics := run("notify", "body-sentinel")
	if code != 4 || output != "" || !strings.Contains(diagnostics, "apprise:phone: failed (provider_failure)") || !strings.Contains(diagnostics, "apprise:chat: failed (provider_failure)") || strings.Contains(diagnostics, "body-sentinel") {
		t.Fatalf("human cascade failure: %d %q %q", code, output, diagnostics)
	}

	t.Setenv("PHONE_EXIT", "3")
	t.Setenv("CHAT_EXIT", "0")
	code, output, diagnostics = run("notify", "body-sentinel")
	if code != 0 || output != "" || diagnostics != "" {
		t.Fatalf("accepted cascade narrated skipped route: %d %q %q", code, output, diagnostics)
	}
}

// Doctor preserves step order and treats an unavailable cascade step as local
// capability information, not a command error when a later step is ready.
func TestDoctorCascadeReadiness(t *testing.T) {
	environment(t)
	t.Setenv("PATH", t.TempDir())
	writeUserConfig(t, "notify:\n  via:\n    - apprise: {tag: phone}\n    - stderr\n")
	code, output, diagnostics := run("--json", "doctor")
	var report struct {
		Ready bool                 `json:"ready"`
		Steps []delivery.Readiness `json:"steps"`
	}
	if json.Unmarshal([]byte(output), &report) != nil || code != 0 || diagnostics != "" || !report.Ready || len(report.Steps) != 2 || report.Steps[0].Label != "apprise:phone" || report.Steps[0].Reason != "missing_executable" || !report.Steps[1].Ready {
		t.Fatalf("doctor cascade report: %d %s %s", code, output, diagnostics)
	}
	code, output, diagnostics = run("doctor")
	if code != 0 || diagnostics != "" || !strings.Contains(output, "apprise:phone (selected): unavailable here") || !strings.Contains(output, "stderr (selected): ready") {
		t.Fatalf("doctor human cascade: %d %s %s", code, output, diagnostics)
	}
}

// Single-route tag behavior must suppress inherited APPRISE_URLS as well.
func TestSingleRouteAppriseTagStripsInheritedURLs(t *testing.T) {
	environment(t)
	record := provider(t)
	writeExecutable(t, filepath.Join(filepath.Dir(record), "apprise"), `#!/bin/sh
printf '%s\000' "$@" > "$FIXTURE_RECORD.args"
printf '%s' "${APPRISE_URLS-unset}" > "$FIXTURE_RECORD.env"
exit 0
`)
	code, output, diagnostics := run("--json", "notify", "--via", "apprise", "--apprise-tag", "selected", "body-sentinel")
	if code != 0 || diagnostics != "" || strings.Contains(output+diagnostics, "credential://") {
		t.Fatalf("tagged apprise result: %d %s %s", code, output, diagnostics)
	}
	got, err := os.ReadFile(record + ".env")
	if err != nil || string(got) != "unset" {
		t.Fatalf("inherited APPRISE_URLS remained: %q %v", got, err)
	}
}

// Missing config is classified before provider execution and doctor reports it.
func TestAppriseMissingConfigIsNotSent(t *testing.T) {
	_ = environment(t)
	record := provider(t)
	code, output, diagnostics := run("--json", "notify", "--via", "apprise", "--apprise-config", "absent.yaml", "body-sentinel")
	assertResult(t, code, output, diagnostics, "apprise", "not_sent", "missing_config")
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("missing Apprise config launched provider")
	}
	code, output, diagnostics = run("--json", "doctor", "--via", "apprise", "--apprise-config", "absent.yaml")
	if code != 1 || diagnostics != "" || !strings.Contains(output, `"reason":"missing_config"`) {
		t.Fatalf("doctor missed missing config: %d %s %s", code, output, diagnostics)
	}
}

func installCascadeProvider(t *testing.T, record string) {
	t.Helper()
	writeExecutable(t, filepath.Join(filepath.Dir(record), "apprise"), `#!/bin/sh
case " $* " in
  *" --tag phone "*) tag=phone ;;
  *" --tag chat "*) tag=chat ;;
  *) tag=unknown ;;
esac
printf '%s\000' "$@" > "$FIXTURE_RECORD.$tag.argv"
while [ "$#" -gt 0 ]; do
  if [ "$1" = --tag ]; then tag=$2; shift 2; else shift; fi
done
printf '%s\000' "$@" > "$FIXTURE_RECORD.$tag.args"
printf '%s' "${APPRISE_URLS-unset}" > "$FIXTURE_RECORD.$tag.env"
case "$tag" in
  phone) exit "${PHONE_EXIT:-3}" ;;
  chat) exit "${CHAT_EXIT:-0}" ;;
esac
exit 77
`)
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
}
