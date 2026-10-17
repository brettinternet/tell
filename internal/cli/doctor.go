package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
		for _, route := range r.Routes {
			selected := ""
			if route.Route == r.Selected || route.Route == r.SelectedSend {
				selected = " (selected)"
			}
			count := ""
			if route.Services != nil {
				count = fmt.Sprintf("; services: %d", *route.Services)
			}
			status := "ready"
			if !route.Ready {
				status = "not ready: " + humanReason(route.Route, route.Reason, "")
			}
			if _, err = fmt.Fprintf(out, "%s%s: %s%s\n", route.Route, selected, status, count); err != nil {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, "tell: could not write readiness report")
		return 1
	}
	if !r.Ready {
		return 1
	}
	return 0
}
