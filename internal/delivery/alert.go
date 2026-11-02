package delivery

import (
	"bytes"
	"context"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// alertScript passes the title, body, and wait as argv, never as script text.
const alertScript = `display dialog (item 2 of argv) with title (item 1 of argv) buttons {"Acknowledge"} default button 1 giving up after (item 3 of argv as integer)`

// notifyAlert blocks until someone clicks Acknowledge or wait elapses. The
// timeout covers provider start and teardown; wait is human time on top of it.
func notifyAlert(ctx context.Context, timeout, wait time.Duration, message Message, platform string) *Result {
	const route = "alert"
	if platform != "darwin" && platform != "linux" {
		return NewResult(route, "not_sent", "unsupported_platform", "alerts are not supported on this platform")
	}
	switch desktopPresence(ctx, timeout, platform) {
	case presenceLocked:
		return NewResult(route, "not_sent", "unattended", "screen is locked; nobody can acknowledge an alert")
	case presenceInactive:
		return NewResult(route, "not_sent", "unattended", "desktop session is not on the active console; nobody can acknowledge an alert")
	}
	if platform == "darwin" {
		seconds := strconv.Itoa(int(math.Ceil(wait.Seconds())))
		args := []string{"-e", "on run argv", "-e", alertScript, "-e", "end run", "--", message.Title, message.Body, seconds}
		stdout, stderr := &probeOutput{}, &probeOutput{}
		code, failure := process(ctx, timeout+wait, route, "osascript", args, nil, nil, stdout, stderr)
		if failure != nil {
			return failure
		}
		return dialogResult(code, stdout.buffer.Bytes(), stderr.buffer.Bytes())
	}
	return notifyAlertLinux(ctx, timeout, wait, message)
}

// dialogResult maps osascript's `display dialog` record or error to an outcome.
func dialogResult(code int, stdout, stderr []byte) *Result {
	const route = "alert"
	out := strings.TrimSpace(string(stdout))
	switch {
	case code == 0 && out == "button returned:Acknowledge, gave up:false":
		result := NewResult(route, "accepted", "", "")
		result.Acknowledged = true
		return result
	case code == 0 && strings.HasSuffix(out, "gave up:true"), code != 0 && bytes.Contains(stderr, []byte("(-128)")):
		return NewResult(route, "not_sent", "unacknowledged", "nobody acknowledged the alert before the wait elapsed")
	case code != 0 && bytes.Contains(stderr, []byte("(-1713)")):
		return NewResult(route, "not_sent", "no_user_interaction", "this process cannot show dialogs; run alerts from the logged-in GUI session")
	}
	return NewResult(route, "failed", "provider_failure", "alert provider reported failure")
}

var notificationID = regexp.MustCompile(`^[0-9]+$`)

// notifyAlertLinux shows a persistent notification with one action and closes
// it by ID when wait elapses; notify-send --wait then exits without the action.
func notifyAlertLinux(ctx context.Context, timeout, wait time.Duration, message Message) *Result {
	const route = "alert"
	args := []string{"--wait", "--urgency", "critical", "--expire-time", "0", "--action", "ack=Acknowledge", "--print-id", "--", message.Title, message.Body}
	output := &lockedOutput{}
	done := make(chan struct{})
	var closer sync.WaitGroup
	closer.Add(1)
	go func() {
		defer closer.Done()
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			if id, _, _ := strings.Cut(output.String(), "\n"); notificationID.MatchString(id) {
				_, _ = RunProcess(ctx, timeout, route, "gdbus", []string{"call", "--session",
					"--dest", "org.freedesktop.Notifications", "--object-path", "/org/freedesktop/Notifications",
					"--method", "org.freedesktop.Notifications.CloseNotification", id})
			}
		}
	}()
	code, failure := process(ctx, timeout+wait, route, "notify-send", args, nil, nil, output, nil)
	close(done)
	closer.Wait()
	if failure != nil {
		return failure
	}
	return actionResult(code, output.String())
}

// actionResult maps notify-send --print-id --wait output: the ID line, then the
// invoked action name, if any.
func actionResult(code int, stdout string) *Result {
	const route = "alert"
	if code != 0 {
		return NewResult(route, "failed", "provider_failure", "alert provider reported failure")
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "ack" {
			result := NewResult(route, "accepted", "", "")
			result.Acknowledged = true
			return result
		}
	}
	return NewResult(route, "not_sent", "unacknowledged", "nobody acknowledged the alert before the wait elapsed")
}

// lockedOutput lets the closer read the notification ID while notify-send writes.
type lockedOutput struct {
	mu     sync.Mutex
	output probeOutput
}

func (o *lockedOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.output.Write(p)
}

func (o *lockedOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.output.buffer.String()
}
