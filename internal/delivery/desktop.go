package delivery

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The platform is explicit so both native argv contracts can be exercised with
// local executables on either development/CI host.
func notifyDesktop(ctx context.Context, timeout time.Duration, message Message, platform string, onClick []string) *Result {
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
	switch desktopPresence(ctx, timeout, platform) {
	case presenceLocked:
		return NewResult(route, "not_sent", "unattended", "screen is locked; nobody can see a desktop notification")
	case presenceInactive:
		return NewResult(route, "not_sent", "unattended", "desktop session is not on the active console; nobody can see a desktop notification")
	}
	if helper := desktopHelper(platform); helper != "" {
		return notifyDesktopHelper(ctx, timeout, helper, message, onClick)
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

// desktopHelper returns the installed Tell.app executable, or "" when the
// desktop route should use osascript. `task macos:install` installs it here.
func desktopHelper(platform string) string {
	home, err := os.UserHomeDir()
	if platform != "darwin" || err != nil {
		return ""
	}
	path := filepath.Join(home, "Applications", "Tell.app", "Contents", "MacOS", "Tell")
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return ""
	}
	return path
}

// notifyDesktopHelper sends the message on stdin so it never appears in argv.
// Only click actions, never message text, are stored with the notification.
func notifyDesktopHelper(ctx context.Context, timeout time.Duration, helper string, message Message, onClick []string) *Result {
	const route = "desktop"
	cwd, _ := os.Getwd()
	payload, err := json.Marshal(struct {
		Title   string        `json:"title"`
		Body    string        `json:"body"`
		Actions []clickAction `json:"actions,omitempty"`
	}{message.Title, message.Body, clickActions(captureOrigin(os.Getenv, cwd), onClick)})
	if err != nil {
		return NewResult(route, "failed", "provider_failure", "could not encode desktop notification")
	}
	code, failure := process(ctx, timeout, route, helper, []string{"post"}, strings.NewReader(string(payload)), herdrEnv(), nil, nil)
	if failure != nil {
		return failure
	}
	switch code {
	case 0:
		return NewResult(route, "accepted", "", "")
	case 3:
		return NewResult(route, "not_sent", "permission_denied", "Tell is not allowed to post notifications")
	default:
		return NewResult(route, "failed", "provider_failure", "desktop notification provider reported failure")
	}
}

// desktopOrigin identifies where a notification came from. It holds only
// identifiers from Tell's own environment, never message content.
type desktopOrigin struct {
	App, HerdrWorkspace, HerdrTab, HerdrSocket, Herdr, CWD string
}

func captureOrigin(getenv func(string) string, cwd string) desktopOrigin {
	origin := desktopOrigin{
		App:            getenv("__CFBundleIdentifier"),
		HerdrWorkspace: getenv("HERDR_WORKSPACE_ID"),
		HerdrTab:       getenv("HERDR_TAB_ID"),
		HerdrSocket:    getenv("HERDR_SOCKET_PATH"),
		CWD:            cwd,
	}
	if origin.HerdrWorkspace != "" || origin.HerdrTab != "" {
		// The click relaunch runs with launchd's minimal PATH, so store an
		// absolute Herdr path.
		origin.Herdr = getenv("HERDR_BIN_PATH")
		if !filepath.IsAbs(origin.Herdr) {
			origin.Herdr, _ = exec.LookPath("herdr")
		}
	}
	return origin
}

// clickAction is one literal argv the helper runs, in order, when the
// notification is clicked. Env entries are added to the helper's environment.
type clickAction struct {
	Argv []string `json:"argv"`
	Env  []string `json:"env,omitempty"`
	Dir  string   `json:"dir,omitempty"`
}

// clickActions activates the origin terminal, then focuses its Herdr workspace
// and tab. A personal on_click command replaces that default and receives the
// origin only as TELL_ORIGIN_* variables.
func clickActions(origin desktopOrigin, onClick []string) []clickAction {
	if len(onClick) > 0 {
		argv := append([]string{}, onClick...)
		if path, err := exec.LookPath(argv[0]); err == nil {
			argv[0], _ = filepath.Abs(path)
		}
		return []clickAction{{Argv: argv, Dir: origin.CWD, Env: []string{
			"TELL_ORIGIN_APP=" + origin.App,
			"TELL_ORIGIN_HERDR_WORKSPACE=" + origin.HerdrWorkspace,
			"TELL_ORIGIN_HERDR_TAB=" + origin.HerdrTab,
			"TELL_ORIGIN_CWD=" + origin.CWD,
		}}}
	}
	var actions []clickAction
	if origin.App != "" {
		actions = append(actions, clickAction{Argv: []string{"/usr/bin/open", "-b", origin.App}})
	}
	if origin.Herdr != "" {
		var env []string
		if origin.HerdrSocket != "" {
			env = []string{"HERDR_SOCKET_PATH=" + origin.HerdrSocket}
		}
		if origin.HerdrWorkspace != "" {
			actions = append(actions, clickAction{Argv: []string{origin.Herdr, "workspace", "focus", origin.HerdrWorkspace}, Env: env})
		}
		if origin.HerdrTab != "" {
			actions = append(actions, clickAction{Argv: []string{origin.Herdr, "tab", "focus", origin.HerdrTab}, Env: env})
		}
	}
	return actions
}
