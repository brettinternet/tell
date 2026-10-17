package delivery

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// ValidRecipient deliberately supports exact local hcom names, not remote/tag
// selectors. Herdr also accepts explicit pane IDs such as w1:p2.
func ValidRecipient(route, to string) bool {
	if config.ValidIdentity(to) && !strings.HasSuffix(to, "-") && to != "help" && to != "self" {
		return true
	}
	if route == "herdr" {
		workspace, pane, ok := strings.Cut(to, ":")
		return ok && strings.HasPrefix(workspace, "w") && strings.HasPrefix(pane, "p") && config.ValidIdentity(workspace) && config.ValidIdentity(pane)
	}
	return false
}

func sendTarget(cfg config.Config) *Result {
	route := cfg.Send.Via
	if route != "hcom" && route != "herdr" {
		return NewResult(route, "not_sent", "missing_route", "select send.via in personal configuration")
	}
	if cfg.Send.To == "" {
		return NewResult(route, "not_sent", "missing_target", "send requires one explicit recipient")
	}
	if !ValidRecipient(route, cfg.Send.To) {
		return NewResult(route, "not_sent", "invalid_target", "send requires one exact recipient, without selectors or @")
	}
	if route == "herdr" && cfg.Herdr.Session == "" {
		return NewResult(route, "not_sent", "missing_session", "Herdr send requires an explicit herdr.session")
	}
	return nil
}

// Send executes once. The caller owns schedules, retries and completion tracking.
func Send(ctx context.Context, cfg config.Config, body, intent string) (result *Result) {
	ctx = childContext(ctx, cfg)
	route := cfg.Send.Via
	redactions := 0
	defer func() { result.Redactions = redactions }()
	if failure := sendTarget(cfg); failure != nil {
		return failure
	}
	if !config.ValidIdentity(cfg.Send.From) || intent != "inform" && intent != "request" {
		return NewResult(route, "config", "invalid_sender", "invalid sender or intent")
	}
	message, count, reason := prepare(Message{Body: body}, cfg.Secrets.Policy)
	redactions = count
	if reason != "" {
		return NewResult(route, "config", reason, "send rejected during message preparation")
	}
	header := "[Tell automation: " + cfg.Send.From + "; route: " + route + "; intent: " + intent + "]\n"
	// The sender is personal configuration, but provenance is still outbound
	// text: scan it too rather than allowing a token-shaped identity to leak.
	provenance, headerCount, headerReason := prepare(Message{Body: header}, cfg.Secrets.Policy)
	redactions += headerCount
	if headerReason != "" {
		return NewResult(route, "config", headerReason, "send rejected during provenance preparation")
	}
	message.Body = provenance.Body + normalize(message.Body, bodyLimit-len(provenance.Body))
	timeout, _ := time.ParseDuration(cfg.Timeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target, failure := probeSend(ctx, cfg)
	if failure != nil {
		return failure
	}
	if route == "hcom" {
		code, failure := RunProcess(ctx, timeout, route, "hcom", []string{"send", "@" + target, "--from", cfg.Send.From, "--intent", intent, "--json", "--", message.Body})
		if failure != nil {
			return failure
		}
		if code != 0 {
			return NewResult(route, "failed", "provider_failure", "hcom send failed")
		}
	} else {
		response, failure := herdrAgent(ctx, cfg, []string{"agent", "prompt", target, message.Body})
		if failure != nil {
			return failure
		}
		if response.Result == nil {
			return NewResult(route, "uncertain", "invalid_response", "Herdr returned no receipt; delivery may have occurred")
		}
	}
	return NewResult(route, "accepted", "", "")
}

// probeSend is read-only, so an interrupted probe means nothing was sent:
// report it as such rather than as uncertain delivery.
func probeSend(ctx context.Context, cfg config.Config) (string, *Result) {
	target, failure := probeTarget(ctx, cfg)
	if failure == nil || failure.Outcome != "uncertain" {
		return target, failure
	}
	if failure.Failure.Reason == "cancelled" {
		return "", NewResult(failure.Route, "config", "cancelled", "send cancelled before delivery")
	}
	return "", NewResult(failure.Route, "not_sent", failure.Failure.Reason, "recipient check did not complete; nothing was sent")
}

// probeTarget: hcom requires an exact listed active name; Herdr requires
// idle/done and returns the resolved pane ID to pin the prompt target.
func probeTarget(ctx context.Context, cfg config.Config) (string, *Result) {
	if failure := sendTarget(cfg); failure != nil {
		return "", failure
	}
	route := cfg.Send.Via
	if route == "herdr" {
		response, failure := herdrAgent(ctx, cfg, []string{"agent", "get", cfg.Send.To})
		if failure != nil {
			return "", failure
		}
		if response.Result == nil || response.Result.Agent == nil {
			return "", NewResult(route, "not_sent", "invalid_response", "invalid Herdr agent state")
		}
		agent := response.Result.Agent
		switch agent.Status {
		case "idle", "done":
			if !ValidRecipient("herdr", agent.PaneID) || !strings.Contains(agent.PaneID, ":") {
				return "", NewResult(route, "not_sent", "invalid_response", "invalid Herdr pane identity")
			}
			return agent.PaneID, nil
		case "blocked":
			return "", NewResult(route, "not_sent", "blocked", "agent is blocked")
		default:
			return "", NewResult(route, "not_sent", "busy", "agent is not known to be idle or done")
		}
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	output := &probeOutput{}
	code, failure := process(ctx, timeout, route, "hcom", []string{"list", "--json"}, nil, nil, output, nil)
	if failure != nil {
		return "", failure
	}
	if code != 0 {
		return "", NewResult(route, "failed", "provider_failure", "hcom readiness probe failed")
	}
	var agents []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if output.overflow || json.Unmarshal(output.buffer.Bytes(), &agents) != nil || agents == nil {
		return "", NewResult(route, "not_sent", "invalid_response", "invalid hcom agent listing")
	}
	matches := 0
	for _, agent := range agents {
		// hcom's mailbox can receive while active or blocked; unlike a typed
		// Herdr turn this cannot interrupt it. Unknown/new states fail closed.
		if agent.Name == cfg.Send.To && (agent.Status == "active" || agent.Status == "listening" || agent.Status == "blocked") {
			matches++
		}
	}
	if matches != 1 {
		return "", NewResult(route, "not_sent", "missing_target", "no unique active hcom recipient")
	}
	return cfg.Send.To, nil
}

type herdrResponse struct {
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
	Result *struct {
		Agent *struct {
			Status string `json:"agent_status"`
			PaneID string `json:"pane_id"`
		} `json:"agent"`
	} `json:"result"`
}

func herdrAgent(ctx context.Context, cfg config.Config, args []string) (herdrResponse, *Result) {
	const route = "herdr"
	timeout, _ := time.ParseDuration(cfg.Timeout)
	output := &probeOutput{}
	code, failure := process(ctx, timeout, route, "herdr", append([]string{"--session", cfg.Herdr.Session}, args...), nil, herdrEnv(), output, nil)
	var response herdrResponse
	if failure != nil {
		return response, failure
	}
	if output.overflow || json.Unmarshal(output.buffer.Bytes(), &response) != nil {
		if code != 0 {
			return response, NewResult(route, "failed", "provider_failure", "Herdr agent operation failed")
		}
		outcome := "not_sent"
		if args[1] == "prompt" {
			outcome = "uncertain"
		}
		return response, NewResult(route, outcome, "invalid_response", "invalid Herdr response")
	}
	if response.Error != nil {
		outcome, reason := "failed", "provider_failure"
		switch response.Error.Code {
		case "agent_blocked":
			outcome, reason = "not_sent", "blocked"
		case "agent_not_found":
			outcome, reason = "not_sent", "missing_target"
		case "timeout":
			outcome, reason = "uncertain", "timeout"
		}
		return response, NewResult(route, outcome, reason, "Herdr agent operation did not succeed")
	}
	if code != 0 {
		return response, NewResult(route, "failed", "provider_failure", "Herdr agent operation failed")
	}
	return response, nil
}
