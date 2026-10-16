package delivery

import (
	"context"
	"os"
	"strings"
	"time"
)

func notifyHerdr(ctx context.Context, timeout time.Duration, session string, message Message) *Result {
	const route = "herdr"
	if session == "" {
		return NewResult(route, "not_sent", "missing_target", "Herdr notifications require an explicit herdr.session")
	}
	// Herdr 0.9.5 scans global options even inside notification payloads.
	// Its notification parser has no positional -- escape. Fail closed rather
	// than let a message change the session or report help as delivery.
	for _, text := range []string{message.Title, message.Body} {
		for _, option := range []string{"--session", "--remote", "--remote-keybindings", "--handoff"} {
			if text == option || strings.HasPrefix(text, option+"=") {
				return NewResult(route, "not_sent", "unsupported_message", "message conflicts with reserved Herdr CLI options")
			}
		}
	}
	if message.Title == "help" || message.Title == "--help" || message.Title == "-h" {
		return NewResult(route, "not_sent", "unsupported_message", "title conflicts with reserved Herdr CLI help")
	}
	// Do not carry another pane's session/socket/config routing into the child.
	// HOME/XDG_CONFIG_HOME still locate the user's normal Herdr installation.
	env := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HERDR_") {
			env = append(env, entry)
		}
	}
	args := []string{"--session", session, "notification", "show", message.Title, "--body", message.Body, "--sound", "none"}
	code, failure := runProcess(ctx, timeout, route, "herdr", args, nil, env)
	if failure != nil {
		return failure
	}
	if code != 0 {
		return NewResult(route, "failed", "provider_failure", "Herdr notification failed; check the configured session is running")
	}
	return NewResult(route, "accepted", "", "")
}
