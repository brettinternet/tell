package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// Readiness contains only fixed diagnostics, never provider output or settings.
type Readiness struct {
	Route    string `json:"route"`
	Ready    bool   `json:"ready"`
	Reason   string `json:"reason"`
	Services *int   `json:"services,omitempty"`
}

// Doctor checks the selected route and routes with explicit configuration.
// It never delivers, starts services, or executes a personal command.
func Doctor(ctx context.Context, cfg config.Config) []Readiness {
	ctx = childContext(ctx, cfg)
	routes := []string{cfg.Notify.Via}
	for _, entry := range []struct {
		route      string
		configured bool
	}{
		{"apprise", cfg.Apprise.Config != "" || cfg.Apprise.Tag != "" || cfg.Apprise.Format != ""},
		{"herdr", cfg.Herdr.Session != ""},
		{"command", len(cfg.Notify.Command) > 0},
	} {
		if entry.configured && entry.route != cfg.Notify.Via {
			routes = append(routes, entry.route)
		}
	}
	results := make([]Readiness, 0, len(routes))
	for _, route := range routes {
		results = append(results, checkRoute(ctx, cfg, route, runtime.GOOS))
	}
	if cfg.Send.Via != "" || cfg.Send.To != "" {
		_, failure := probeSend(ctx, cfg)
		result := Readiness{Route: "send/" + cfg.Send.Via, Ready: failure == nil, Reason: "ready"}
		if failure != nil {
			result.Reason = failure.Failure.Reason
		}
		results = append(results, result)
	}
	return results
}

func checkRoute(ctx context.Context, cfg config.Config, route, platform string) Readiness {
	result := Readiness{Route: route, Reason: "ready"}
	finish := func(reason string) Readiness { result.Reason = reason; result.Ready = reason == "ready"; return result }
	if ctx.Err() != nil {
		return finish("cancelled")
	}
	var name string
	switch route {
	case "stderr":
		return finish("ready")
	case "command":
		if len(cfg.Notify.Command) == 0 {
			return finish("missing_command")
		}
		if _, err := exec.LookPath(cfg.Notify.Command[0]); err != nil {
			return finish("command_not_executable")
		}
		return finish("ready")
	case "desktop":
		switch platform {
		case "darwin":
			name = "osascript"
		case "linux":
			name = "notify-send"
		default:
			return finish("unsupported_platform")
		}
	case "herdr":
		if cfg.Herdr.Session == "" {
			return finish("missing_target")
		}
		name = "herdr"
	case "apprise":
		name = "apprise"
	default:
		return finish("unsupported_route")
	}
	if _, err := exec.LookPath(name); err != nil {
		return finish("missing_executable")
	}
	if route == "desktop" {
		return finish("ready")
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"session", "list", "--json"}
	if route == "apprise" {
		args = []string{"--dry-run"}
		for _, option := range []struct{ flag, value string }{
			{"--config", cfg.Apprise.Config}, {"--tag", cfg.Apprise.Tag}, {"--input-format", cfg.Apprise.Format},
		} {
			if option.value != "" {
				args = append(args, option.flag, option.value)
			}
		}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if route == "herdr" {
		cmd.Env = herdrEnv()
	}
	cmd.Env = childEnv(ctx, cmd.Env)
	cmd.WaitDelay = 100 * time.Millisecond
	output := &probeOutput{}
	cmd.Stdout = output // stderr is discarded; stdin receives EOF.
	err := cmd.Run()
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return finish("timeout")
		}
		return finish("cancelled")
	}
	if output.overflow {
		return finish("invalid_response")
	}
	if route == "apprise" {
		count := len(appriseEntry.FindAll(output.buffer.Bytes(), -1))
		result.Services = &count
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 3 {
				return finish("no_services")
			}
			return finish("probe_failed")
		}
		if count == 0 {
			return finish("no_services")
		}
		return finish("ready")
	}
	if err != nil {
		return finish("probe_failed")
	}
	var listing struct {
		Sessions *[]struct {
			Name    string `json:"name"`
			Running bool   `json:"running"`
		} `json:"sessions"`
	}
	if json.Unmarshal(output.buffer.Bytes(), &listing) != nil || listing.Sessions == nil {
		return finish("invalid_response")
	}
	for _, session := range *listing.Sessions {
		if session.Name == cfg.Herdr.Session && session.Running {
			return finish("ready")
		}
	}
	return finish("session_not_running")
}

// Apprise dry-run prints numbered URLs, followed by indented metadata. Retain
// only their count; neither the URLs nor metadata escape this package.
var appriseEntry = regexp.MustCompile(`(?m)^ *[0-9]+\. \S+`)

// Bound captured output while continuing to drain the subprocess pipe.
type probeOutput struct {
	buffer   bytes.Buffer // Named field: embedding would promote ReadFrom and bypass Write.
	overflow bool
}

func (b *probeOutput) Write(p []byte) (int, error) {
	n := len(p)
	const limit = 1024 * 1024
	if len(p) > limit-b.buffer.Len() {
		p = p[:limit-b.buffer.Len()]
		b.overflow = true
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}

func herdrEnv() []string {
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HERDR_") {
			env = append(env, entry)
		}
	}
	return env
}
