package delivery

import (
	"context"
	"time"
)

// The platform is explicit so both native argv contracts can be exercised with
// local executables on either development/CI host.
func notifyDesktop(ctx context.Context, timeout time.Duration, message Message, platform string) *Result {
	const route = "desktop"
	var name string
	var args []string
	switch platform {
	case "darwin":
		name = "osascript"
		args = []string{"-e", "on run argv", "-e",
			"display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run", "--", message.Title, message.Body}
	case "linux":
		name = "notify-send"
		urgency := "normal"
		if message.Type == "failure" {
			urgency = "critical"
		}
		args = []string{"--urgency", urgency, "--", message.Title, message.Body}
	default:
		return NewResult(route, "not_sent", "unsupported_platform", "desktop notifications are not supported on this platform")
	}
	code, failure := RunProcess(ctx, timeout, route, name, args)
	if failure != nil {
		return failure
	}
	if code != 0 {
		return NewResult(route, "failed", "provider_failure", "desktop notification provider reported failure")
	}
	return NewResult(route, "accepted", "", "")
}
