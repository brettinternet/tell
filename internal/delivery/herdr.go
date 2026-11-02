package delivery

import (
	"context"
	"encoding/json"
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
	env := herdrEnv()
	args := []string{"--session", session, "notification", "show", message.Title, "--body", message.Body, "--sound", "none"}
	output := &probeOutput{}
	code, failure := process(ctx, timeout, route, "herdr", args, nil, env, output, nil)
	if failure != nil {
		return failure
	}
	return herdrShowResult(code, output)
}

// herdrShowResult maps `herdr notification show` to an outcome. Herdr exits 0
// even when it declines to show, so its reason decides whether the cascade
// advances. Unparseable output keeps the exit-code meaning.
func herdrShowResult(code int, output *probeOutput) *Result {
	const route = "herdr"
	if code != 0 {
		return NewResult(route, "failed", "provider_failure", "Herdr notification failed; check the configured session is running")
	}
	var response struct {
		Result *struct {
			Reason string `json:"reason"`
			Shown  *bool  `json:"shown"`
		} `json:"result"`
	}
	if output.overflow || json.Unmarshal(output.buffer.Bytes(), &response) != nil || response.Result == nil {
		return NewResult(route, "accepted", "", "")
	}
	switch reason := response.Result.Reason; reason {
	case "disabled", "no_foreground_client", "rate_limited", "busy":
		return NewResult(route, "not_sent", reason, "Herdr did not show the notification")
	}
	if shown := response.Result.Shown; shown != nil && !*shown {
		return NewResult(route, "not_sent", "not_shown", "Herdr did not show the notification")
	}
	return NewResult(route, "accepted", "", "")
}
