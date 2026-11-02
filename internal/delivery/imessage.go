package delivery

import (
	"context"
	"regexp"
	"strconv"
	"time"
)

// The recipient and text arrive as argv, never as script source. The account
// lookup raises a fixed error number so an unavailable account is distinguishable
// from other Messages failures without reading its message text.
const (
	imessageNoAccount        = 7001
	imessageAutomationDenied = -1743 // errAEEventNotPermitted
	imessageEventTimeout     = -1712 // errAETimeout
)

var imessageAccount = []string{
	"-e", "on run argv",
	"-e", `tell application "Messages"`,
	"-e", "set candidates to every account whose service type = iMessage and enabled is true",
	"-e", `if candidates is {} then error "no iMessage account" number 7001`,
}

var imessageEnd = []string{"-e", "end tell", "-e", "end run"}

func imessageSendArgs(to string, message Message) []string {
	text := message.Body
	if message.Title != "" {
		text = message.Title + "\n" + text
	}
	args := append(append([]string{}, imessageAccount...), "-e", "send (item 2 of argv) to participant (item 1 of argv) of (item 1 of candidates)")
	return append(append(args, imessageEnd...), "--", to, text)
}

// osascript reports script errors on stderr as "... (NUMBER)". Only the number
// is retained; the captured text never leaves this package.
var appleScriptError = regexp.MustCompile(`\((-?[0-9]+)\)\s*$`)

func runMessages(ctx context.Context, timeout time.Duration, args []string) (int, *Result) {
	output := &probeOutput{}
	code, failure := process(ctx, timeout, "imessage", "osascript", args, nil, nil, nil, output)
	if failure != nil || code == 0 {
		return 0, failure
	}
	match := appleScriptError.FindSubmatch(output.buffer.Bytes())
	if match == nil {
		return 1, nil
	}
	number, err := strconv.Atoi(string(match[1]))
	if err != nil || number == 0 {
		return 1, nil
	}
	return number, nil
}

// notifyIMessage hands one message to Messages. Accepted means Messages took it,
// not that it was delivered or read.
func notifyIMessage(ctx context.Context, timeout time.Duration, to string, message Message, platform string) *Result {
	const route = "imessage"
	if platform != "darwin" {
		return NewResult(route, "not_sent", "unsupported_platform", "iMessage requires macOS")
	}
	if to == "" {
		return NewResult(route, "config", "invalid_config", "imessage route requires imessage.to in personal config")
	}
	number, failure := runMessages(ctx, timeout, imessageSendArgs(to, message))
	switch {
	case failure != nil:
		return failure
	case number == 0:
		return NewResult(route, "accepted", "", "")
	case number == imessageAutomationDenied:
		return NewResult(route, "not_sent", "automation_denied", "Automation permission to control Messages is denied")
	case number == imessageNoAccount:
		return NewResult(route, "not_sent", "no_account", "Messages has no enabled iMessage account")
	case number == imessageEventTimeout:
		return NewResult(route, "uncertain", "timeout", "Messages did not answer in time; delivery may have occurred")
	default:
		return NewResult(route, "failed", "provider_failure", "Messages reported failure")
	}
}

// checkIMessage reads the account list without sending. It may launch Messages
// and, on first use, show the Automation permission prompt.
func checkIMessage(ctx context.Context, timeout time.Duration, platform string) string {
	if platform != "darwin" {
		return "unsupported_platform"
	}
	number, failure := runMessages(ctx, timeout, append(append([]string{}, imessageAccount...), imessageEnd...))
	switch {
	case failure != nil:
		switch reason := failure.Failure.Reason; reason {
		case "missing_executable", "timeout", "cancelled":
			return reason
		}
		return "probe_failed"
	case number == 0:
		return "ready"
	case number == imessageAutomationDenied:
		return "automation_denied"
	case number == imessageNoAccount:
		return "no_account"
	default:
		return "probe_failed"
	}
}
