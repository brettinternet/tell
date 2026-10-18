package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

type doctorReport struct {
	SchemaVersion int                  `json:"schema_version"`
	Selected      string               `json:"selected"`
	SelectedSend  string               `json:"selected_send,omitempty"`
	Ready         bool                 `json:"ready"`
	Routes        []delivery.Readiness `json:"routes"`
}

func (*doctorReport) Error() string { return "route readiness report" }

func doctor(ctx context.Context, cmd *urfave.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	cfg, err := loadSettings(cmd)
	if err != nil {
		return err
	}
	routes := delivery.Doctor(ctx, cfg)
	ready := routes[0].Ready
	selectedSend := ""
	if cfg.Send.Via != "" || cfg.Send.To != "" {
		selectedSend = "send/" + cfg.Send.Via
		ready = ready && routes[len(routes)-1].Ready
	}
	if cfg.Listen.Slack != nil {
		inbound := delivery.Readiness{Route: "listen/slack", Ready: os.Getenv(cfg.SlackTokenEnv()) != "", Reason: "local_config_only"}
		if !inbound.Ready {
			inbound.Reason = "missing_listener_credential"
		}
		routes = append(routes, inbound)
	}
	if cfg.Listen.GitHub != nil {
		inbound := delivery.Readiness{Route: "listen/github", Ready: os.Getenv(cfg.GitHubSecretEnv()) != "", Reason: "local_config_only"}
		if !inbound.Ready {
			inbound.Reason = "missing_listener_credential"
		}
		routes = append(routes, inbound)
	}
	return &doctorReport{SchemaVersion: 1, Selected: cfg.Notify.Via, SelectedSend: selectedSend, Ready: ready, Routes: routes}
}

// humanReason adds CLI-only guidance without changing structured provider results.
// Route and reason select fixed text; never include provider output or settings.
func humanReason(route, reason, fallback string) string {
	switch reason {
	case "missing_route":
		return "choose send.via in personal config, --send-via, or TELL_SEND_VIA (hcom or herdr)"
	case "missing_target":
		if strings.HasPrefix(route, "send/") {
			return "no unique recipient is available; set send.to in personal config, --to, or TELL_SEND_TO to one existing agent"
		}
		fallthrough
	case "missing_session":
		return "choose an explicit Herdr session with herdr.session in personal config, --herdr-session, or TELL_HERDR_SESSION"
	case "missing_executable":
		name := strings.TrimPrefix(route, "send/")
		if name == "desktop" {
			name = "notify-send"
			if runtime.GOOS == "darwin" {
				name = "osascript"
			}
		}
		if name == "command" {
			return "the notify.command executable was not found; check its path in personal config"
		}
		return name + " was not found on PATH; install it or choose another route"
	}
	if fallback != "" {
		return fallback
	}
	switch reason {
	case "missing_listener_credential":
		if route == "listen/github" {
			return "set the configured listen.github.secret_env variable; readiness is local only, not authentication"
		}
		return "set the configured listen.slack.app_token_env variable; readiness is local only, not authentication"
	case "missing_command":
		return "set notify.command to an executable and arguments in personal config"
	case "command_not_executable":
		return "notify.command cannot be executed; check its path and permissions in personal config"
	case "unsupported_platform":
		return "desktop notifications require macOS or Linux; choose another route"
	case "unsupported_route":
		return "this route is not supported; choose another route"
	case "cancelled":
		return "readiness check was cancelled; run tell doctor again"
	case "timeout":
		return "readiness check timed out; check the provider or increase --timeout"
	case "invalid_response":
		return "the provider returned an invalid readiness response; check its version and setup"
	case "no_services":
		return "Apprise matched no services; check its provider config and apprise.tag"
	case "probe_failed", "provider_failure":
		return "the provider readiness check failed; check its setup"
	case "session_not_running":
		return "the selected Herdr session is not running; start it or select a running session with --herdr-session"
	case "invalid_target":
		return "set send.to (--to or TELL_SEND_TO) to one exact recipient without selectors or @"
	case "busy":
		return "the agent is not idle or done; wait for it to finish before sending"
	case "blocked":
		return "the agent is blocked; resolve its blocker before sending"
	case "start_failed":
		return "the provider executable could not start; check its permissions and setup"
	default:
		return "readiness check did not succeed; check the provider setup"
	}
}

func (r *doctorReport) render(root *urfave.Command, args []string, out, errOut io.Writer) int {
	var err error
	if jsonRequested(root, args) {
		err = json.NewEncoder(out).Encode(r)
	} else {
		style := newOutputStyle(out, boolRequested(root, args, "no-color"))
		for _, route := range r.Routes {
			selected := ""
			if route.Route == r.Selected || route.Route == r.SelectedSend {
				selected = " (selected)"
			}
			count := ""
			if strings.HasPrefix(route.Route, "listen/") {
				count = "; local configuration only, not authentication"
			}
			if route.Services != nil {
				count = fmt.Sprintf("; services: %d", *route.Services)
			}
			label := route.Route + selected + ":"
			if selected != "" {
				label = style.bold(label)
			}
			status := style.success("ready")
			if !route.Ready {
				status = "not ready"
				if selected != "" {
					status = style.failure(status)
				}
			}
			if _, err = fmt.Fprintf(out, "%s %s%s\n", label, status, count); err != nil {
				break
			}
			if !route.Ready {
				if _, err = fmt.Fprintf(out, "  %s\n", humanReason(route.Route, route.Reason, "")); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		style := newOutputStyle(errOut, jsonRequested(root, args) || boolRequested(root, args, "no-color"))
		fmt.Fprintf(errOut, "%s could not write readiness report\n", style.failure("tell:"))
		return 1
	}
	if !r.Ready {
		return 1
	}
	return 0
}
