package delivery

import (
	"context"
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
			if _, err := os.Stat(record); !os.IsNotExist(err) {
				t.Fatal("desktop readiness executed provider")
			}
			for _, kind := range []string{"info", "success", "warning", "failure"} {
				message := Message{Title: "-title ' \" \\\nnext", Body: "--body ' \" \\\n$(no-shell)", Type: kind}
				result := notifyDesktop(context.Background(), time.Second, message, platform)
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
			result := notifyDesktop(context.Background(), time.Second, Message{Body: "body"}, platform)
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
				result := notifyDesktop(context.Background(), time.Second, Message{Body: "body"}, platform)
				readiness := checkRoute(context.Background(), config.Config{Timeout: "1s"}, "desktop", platform)
				_, err := os.Stat(record)
				if name == "unattended" {
					if result.Outcome != "not_sent" || result.Failure.Reason != "unattended" || !os.IsNotExist(err) {
						t.Fatalf("%+v sent=%v", result, err == nil)
					}
					if readiness.Ready || readiness.Reason != "unattended" || readiness.Presence != presenceLocked {
						t.Fatalf("readiness: %+v", readiness)
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
