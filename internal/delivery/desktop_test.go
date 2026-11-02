package delivery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// Guard both platforms against source/option injection even on single-OS CI.
func TestDesktopLiteralArguments(t *testing.T) {
	for _, platform := range []string{"darwin", "linux", "windows"} {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			record := filepath.Join(dir, "args")
			t.Setenv("PATH", dir)
			t.Setenv("HOME", dir) // no Tell.app: the osascript fallback is unchanged
			t.Setenv("FIXTURE_RECORD", record)
			for _, name := range []string{"osascript", "notify-send"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nprintf '%s\\000' \"$@\" > \"$FIXTURE_RECORD\"\n"), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			readiness := checkRoute(context.Background(), config.Config{}, "desktop", platform)
			if readiness.Ready != (platform != "windows") || (platform == "windows" && readiness.Reason != "unsupported_platform") {
				t.Fatalf("readiness: %+v", readiness)
			}
			if platform == "darwin" && (readiness.Provider != "osascript" || readiness.Permission != "") {
				t.Fatalf("fallback provider: %+v", readiness)
			}
			if _, err := os.Stat(record); !os.IsNotExist(err) {
				t.Fatal("desktop readiness executed provider")
			}
			for _, kind := range []string{"info", "success", "warning", "failure"} {
				message := Message{Title: "-title ' \" \\\nnext", Body: "--body ' \" \\\n$(no-shell)", Type: kind}
				result := notifyDesktop(context.Background(), time.Second, message, platform, nil)
				if platform == "windows" {
					if result.Outcome != "not_sent" || result.Failure.Reason != "unsupported_platform" {
						t.Fatalf("%+v", result)
					}
					if _, err := os.Stat(record); !os.IsNotExist(err) {
						t.Fatal("unsupported platform started provider")
					}
					continue
				}
				if result.Outcome != "accepted" {
					t.Fatalf("%+v", result)
				}
				data, err := os.ReadFile(record)
				if err != nil {
					t.Fatal(err)
				}
				want := []string{"-e", "on run argv", "-e", "display notification (item 2 of argv) with title (item 1 of argv)", "-e", "end run", "--", message.Title, message.Body}
				if platform == "linux" {
					urgency := "normal"
					if kind == "failure" {
						urgency = "critical"
					}
					want = []string{"--urgency", urgency, "--", message.Title, message.Body}
				}
				got := strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("argv: %q want %q", got, want)
				}
			}
			t.Setenv("PATH", t.TempDir())
			result := notifyDesktop(context.Background(), time.Second, Message{Body: "body"}, platform, nil)
			if result.Outcome != "not_sent" {
				t.Fatalf("%+v", result)
			}
			if platform != "windows" && result.Failure.Reason != "missing_executable" {
				t.Fatalf("%+v", result)
			}
		})
	}
}

// Unattended desktops must advance a cascade; unreadable presence must still send.
func TestDesktopPresence(t *testing.T) {
	user := func(uid string, extra string) string {
		return "<dict><key>kCGSSessionUserIDKey</key><integer>" + uid + "</integer>" + extra + "</dict>"
	}
	plist := func(locked, users string) string {
		return `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>IOConsoleLocked</key><` + locked +
			`/><key>IOConsoleUsers</key><array>` + users + `</array><key>IOKitDiagnostics</key><dict><key>Classes</key><dict><key>X</key><integer>1</integer></dict></dict></dict></plist>`
	}
	on := "<key>kCGSSessionOnConsoleKey</key><true/>"
	off := "<key>kCGSSessionOnConsoleKey</key><false/>"
	for name, test := range map[string]struct{ out, want string }{
		"attended":       {plist("false", user("7", "")+user("501", on)), presenceAttended},
		"locked":         {plist("true", user("501", on)), presenceLocked},
		"legacy locked":  {plist("false", user("501", on+"<key>CGSSessionScreenIsLocked</key><true/>")), presenceLocked},
		"switched away":  {plist("false", user("501", off)+user("7", on)), presenceInactive},
		"no user entry":  {plist("false", user("7", on)), presenceUnknown},
		"no console key": {plist("false", user("501", "")), presenceUnknown},
		"malformed":      {"<plist><dict><key>IOConsoleLocked</key><true/>", presenceUnknown},
		"empty":          {"", presenceUnknown},
	} {
		if got := parseConsolePresence([]byte(test.out), 501); got != test.want {
			t.Errorf("ioreg %s: %s want %s", name, got, test.want)
		}
	}
	for out, want := range map[string]string{
		"Active=yes\nLockedHint=no\n":  presenceAttended,
		"Active=yes\nLockedHint=yes\n": presenceLocked,
		"Active=no\nLockedHint=no\n":   presenceInactive,
		"":                             presenceUnknown,
	} {
		if got := parseLogindPresence([]byte(out)); got != want {
			t.Errorf("loginctl %q: %s want %s", out, got, want)
		}
	}

	locked := map[string]string{
		"darwin": plist("true", user(strconv.Itoa(os.Getuid()), on)),
		"linux":  "Display=c2\nActive=yes\nLockedHint=yes\n",
	}
	for _, platform := range []string{"darwin", "linux"} {
		for name, probe := range map[string]string{
			"unattended":  "printf '%s' \"$FIXTURE_PRESENCE\"",
			"probe fails": "exit 1",
			"garbage":     "echo garbage",
		} {
			t.Run(platform+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				record := filepath.Join(dir, "sent")
				t.Setenv("PATH", dir)
				t.Setenv("HOME", dir)
				t.Setenv("FIXTURE_RECORD", record)
				t.Setenv("FIXTURE_PRESENCE", locked[platform])
				write := func(name, body string) {
					if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
						t.Fatal(err)
					}
				}
				write("osascript", `: > "$FIXTURE_RECORD"`)
				write("notify-send", `: > "$FIXTURE_RECORD"`)
				write("ioreg", probe)
				write("loginctl", probe)
				result := notifyDesktop(context.Background(), time.Second, Message{Body: "body"}, platform, nil)
				readiness := checkRoute(context.Background(), config.Config{Timeout: "1s"}, "desktop", platform)
				_, err := os.Stat(record)
				if name == "unattended" {
					if result.Outcome != "not_sent" || result.Failure.Reason != "unattended" || !os.IsNotExist(err) {
						t.Fatalf("%+v sent=%v", result, err == nil)
					}
					if readiness.Ready || readiness.Reason != "unattended" || readiness.Presence != presenceLocked {
						t.Fatalf("readiness: %+v", readiness)
					}
					alert := notifyAlert(context.Background(), time.Second, time.Hour, Message{Body: "body"}, platform)
					if _, err := os.Stat(record); alert.Outcome != "not_sent" || alert.Failure.Reason != "unattended" || !os.IsNotExist(err) {
						t.Fatalf("alert: %+v shown=%v", alert, err == nil)
					}
					return
				}
				if result.Outcome != "accepted" || err != nil {
					t.Fatalf("detection failure suppressed notification: %+v", result)
				}
				if !readiness.Ready || readiness.Presence != presenceUnknown {
					t.Fatalf("readiness: %+v", readiness)
				}
			})
		}
	}
}

// A notification from Noon or a non-terminal parent must not activate or focus
// anything on click; one from a Herdr pane in Ghostty returns to that pane's
// app, workspace, and tab through absolute paths the click relaunch can run.
func TestDesktopOrigin(t *testing.T) {
	none := captureOrigin(func(string) string { return "" }, "/work")
	if actions := clickActions(none, nil); actions != nil {
		t.Fatalf("no-origin click actions: %+v", actions)
	}
	env := map[string]string{
		"__CFBundleIdentifier": "com.mitchellh.ghostty", "HERDR_WORKSPACE_ID": "wQY", "HERDR_TAB_ID": "wQY:t1",
		"HERDR_SOCKET_PATH": "/run/herdr.sock", "HERDR_BIN_PATH": "/opt/herdr",
	}
	origin := captureOrigin(func(key string) string { return env[key] }, "/work")
	socket := []string{"HERDR_SOCKET_PATH=/run/herdr.sock"}
	want := []clickAction{
		{Argv: []string{"/usr/bin/open", "-b", "com.mitchellh.ghostty"}},
		{Argv: []string{"/opt/herdr", "workspace", "focus", "wQY"}, Env: socket},
		{Argv: []string{"/opt/herdr", "tab", "focus", "wQY:t1"}, Env: socket},
	}
	if got := clickActions(origin, nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("origin click actions: %+v", got)
	}
}

// Tell.app receives message text only on stdin. on_click replaces the default
// action, sees the origin only as TELL_ORIGIN_* variables, and never receives
// message text; denied permission must advance a cascade.
func TestDesktopHelper(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "payload")
	helper := filepath.Join(dir, "Applications", "Tell.app", "Contents", "MacOS")
	if err := os.MkdirAll(helper, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		filepath.Join(helper, "Tell"): "case \"$1\" in\npost) cat > \"$FIXTURE_RECORD\"; env > \"$FIXTURE_RECORD.env\"; exit \"${FIXTURE_EXIT:-0}\";;\n" +
			"status) printf '%s\\n' \"$FIXTURE_STATUS\";;\nesac",
		filepath.Join(dir, "on-click"): "exit 0",
	} {
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", dir)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin"+string(os.PathListSeparator)+"/usr/bin")
	t.Setenv("FIXTURE_RECORD", record)
	t.Setenv("__CFBundleIdentifier", "com.mitchellh.ghostty")
	t.Setenv("HERDR_WORKSPACE_ID", "wQY")
	t.Setenv("HERDR_TAB_ID", "wQY:t1")
	t.Setenv("HERDR_BIN_PATH", "/opt/herdr")
	cwd, _ := os.Getwd()

	message := Message{Title: "title-secret", Body: "body-secret"}
	result := notifyDesktop(context.Background(), time.Second, message, "darwin", []string{"on-click", "--flag"})
	if result.Outcome != "accepted" {
		t.Fatalf("%+v", result)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Title, Body string
		Actions     []clickAction
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Title != message.Title || payload.Body != message.Body {
		t.Fatalf("payload %s: %v", data, err)
	}
	want := []clickAction{{Argv: []string{filepath.Join(dir, "on-click"), "--flag"}, Dir: cwd, Env: []string{
		"TELL_ORIGIN_APP=com.mitchellh.ghostty", "TELL_ORIGIN_HERDR_WORKSPACE=wQY", "TELL_ORIGIN_HERDR_TAB=wQY:t1", "TELL_ORIGIN_CWD=" + cwd,
	}}}
	if !reflect.DeepEqual(payload.Actions, want) {
		t.Fatalf("on_click actions: %+v", payload.Actions)
	}
	if actions, _ := json.Marshal(payload.Actions); strings.Contains(string(actions), "secret") {
		t.Fatalf("message text reached click actions: %s", actions)
	}
	if env, _ := os.ReadFile(record + ".env"); strings.Contains(string(env), "HERDR_") || strings.Contains(string(env), "secret") {
		t.Fatalf("helper environment: %s", env)
	}

	t.Setenv("FIXTURE_EXIT", "3")
	if result := notifyDesktop(context.Background(), time.Second, message, "darwin", nil); result.Outcome != "not_sent" || result.Failure.Reason != "permission_denied" {
		t.Fatalf("denied: %+v", result)
	}
	for status, reason := range map[string]string{"authorized": "ready", "not_determined": "ready", "denied": "permission_denied", "garbage": "ready"} {
		t.Setenv("FIXTURE_STATUS", status)
		readiness := checkRoute(context.Background(), config.Config{Timeout: "1s"}, "desktop", "darwin")
		permission := status
		if status == "garbage" {
			permission = "unknown"
		}
		if readiness.Provider != "tell-app" || readiness.Permission != permission || readiness.Reason != reason {
			t.Fatalf("doctor %s: %+v", status, readiness)
		}
	}
}
