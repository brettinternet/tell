package delivery

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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
