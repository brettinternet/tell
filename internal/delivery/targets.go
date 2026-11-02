package delivery

import (
	"context"
	"encoding/json"
	"regexp"
	"runtime"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// Target is one recipient send would accept. Fields hold only the send.to
// value and fixed vocabularies, never raw provider output.
type Target struct {
	To     string `json:"to"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Ready  bool   `json:"ready"`
}

// Targets lists recipients on the configured send route. It is read-only:
// it never sends, prompts, or starts services, and never chooses a recipient.
func Targets(ctx context.Context, cfg config.Config) ([]Target, *Result) {
	ctx = childContext(ctx, cfg)
	route := cfg.Send.Via
	if route != "hcom" && route != "herdr" {
		return nil, NewResult(route, "not_sent", "missing_route", "select send.via in personal configuration")
	}
	if route == "herdr" && cfg.Herdr.Session == "" {
		return nil, NewResult(route, "not_sent", "missing_session", "Herdr listing requires an explicit herdr.session")
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if route == "herdr" {
		return herdrTargets(ctx, cfg)
	}
	output := &probeOutput{}
	code, failure := process(ctx, timeout, route, "hcom", []string{"list", "--json"}, nil, nil, output, nil)
	if failure != nil {
		return nil, readOnlyFailure(failure, "listing cancelled", "recipient listing did not complete")
	}
	if code != 0 {
		return nil, NewResult(route, "failed", "provider_failure", "hcom listing failed")
	}
	var agents []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Tool   string `json:"tool"`
	}
	if output.overflow || json.Unmarshal(output.buffer.Bytes(), &agents) != nil || agents == nil {
		return nil, NewResult(route, "not_sent", "invalid_response", "invalid hcom agent listing")
	}
	targets := []Target{}
	index := map[string]int{}
	for _, agent := range agents {
		// Match probeTarget: only these states can receive; others are not recipients.
		if !ValidRecipient(route, agent.Name) || agent.Status != "active" && agent.Status != "listening" && agent.Status != "blocked" {
			continue
		}
		if i, seen := index[agent.Name]; seen {
			// send refuses a name that is not unique.
			targets[i].Status, targets[i].Ready = "duplicate", false
			continue
		}
		index[agent.Name] = len(targets)
		targets = append(targets, Target{To: agent.Name, Kind: agentKind(agent.Tool), Status: agent.Status, Ready: true})
	}
	return targets, nil
}

func herdrTargets(ctx context.Context, cfg config.Config) ([]Target, *Result) {
	const route = "herdr"
	// Reuse doctor's session probe so a stopped session reports the same reason.
	switch readiness := checkRoute(ctx, cfg, route, runtime.GOOS); readiness.Reason {
	case "ready":
	case "cancelled":
		return nil, NewResult(route, "config", "cancelled", "listing cancelled")
	case "probe_failed":
		return nil, NewResult(route, "failed", "provider_failure", "Herdr session check failed")
	default:
		return nil, NewResult(route, "not_sent", readiness.Reason, "Herdr session is not available")
	}
	response, failure := herdrAgent(ctx, cfg, []string{"agent", "list"})
	if failure != nil {
		return nil, readOnlyFailure(failure, "listing cancelled", "recipient listing did not complete")
	}
	if response.Result == nil || response.Result.Agents == nil {
		return nil, NewResult(route, "not_sent", "invalid_response", "invalid Herdr agent listing")
	}
	targets := []Target{}
	for _, agent := range *response.Result.Agents {
		to := agent.Name
		if !ValidRecipient(route, to) {
			to = agent.PaneID
			if !ValidRecipient(route, to) {
				continue
			}
		}
		target := Target{To: to, Kind: agentKind(agent.Kind), Status: "unknown"}
		switch agent.Status {
		case "idle", "done":
			target.Status, target.Ready = agent.Status, true
		case "working", "blocked":
			target.Status = agent.Status
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// readOnlyFailure reports an interrupted read-only probe as not sent, since
// nothing could have been delivered.
func readOnlyFailure(failure *Result, cancelled, incomplete string) *Result {
	if failure.Outcome != "uncertain" {
		return failure
	}
	if failure.Failure.Reason == "cancelled" {
		return NewResult(failure.Route, "config", "cancelled", cancelled)
	}
	return NewResult(failure.Route, "not_sent", failure.Failure.Reason, incomplete)
}

var agentKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// agentKind passes through only short provider kind identifiers.
func agentKind(kind string) string {
	if agentKindPattern.MatchString(kind) {
		return kind
	}
	return "unknown"
}
