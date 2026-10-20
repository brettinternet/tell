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
	Label    string `json:"label,omitempty"`
	Ready    bool   `json:"ready"`
	Reason   string `json:"reason"`
	Services *int   `json:"services,omitempty"`
}

// Doctor checks the selected route and routes with explicit configuration.
// It never delivers, starts services, or executes a personal command.
func Doctor(ctx context.Context, cfg config.Config) []Readiness {
	ctx = childContext(ctx, cfg)
	steps, err := config.EffectiveNotifySteps(cfg)
	if err != nil || len(steps) == 0 {
		steps = []config.NotifyStep{{Via: cfg.Notify.Via}}
	}
	results := make([]Readiness, 0, len(steps)+3)
	present := make(map[string]bool)
	for _, step := range steps {
		result := checkNotifyStep(ctx, cfg, step, runtime.GOOS)
		result.Label = config.NotifyStepLabel(step)
		if result.Label == result.Route {
			result.Label = ""
		}
		results = append(results, result)
		present[step.Via] = true
	}
	for _, entry := range []struct {
		route      string
		configured bool
	}{
		{"apprise", cfg.Apprise.Config != "" || cfg.Apprise.Tag != "" || cfg.Apprise.Format != ""},
		{"herdr", cfg.Herdr.Session != ""},
		{"command", len(cfg.Notify.Command) > 0},
	} {
		if entry.configured && !present[entry.route] {
			results = append(results, checkRoute(ctx, cfg, entry.route, runtime.GOOS))
		}
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

func checkNotifyStep(ctx context.Context, cfg config.Config, step config.NotifyStep, platform string) Readiness {
	cfg.Notify.Via = step.Via
	cfg.Notify.Steps = nil
	if step.Via == "apprise" {
		cfg.Apprise.Config, cfg.Apprise.Tag, cfg.Apprise.Format = step.Config, step.Tag, step.Format
	}
	if step.Via == "herdr" {
		cfg.Herdr.Session = step.Session
	}
	return checkRouteWithEnv(ctx, cfg, step.Via, platform, step.UrlsEnv)
}

func checkRoute(ctx context.Context, cfg config.Config, route, platform string) Readiness {
	return checkRouteWithEnv(ctx, cfg, route, platform, "")
}

func checkRouteWithEnv(ctx context.Context, cfg config.Config, route, platform, urlsEnv string) Readiness {
	result := Readiness{Route: route, Reason: "ready"}
	finish := func(reason string) Readiness { result.Reason = reason; result.Ready = reason == "ready"; return result }
	if ctx.Err() != nil {
		return finish("cancelled")
	}
	if route == "apprise" {
		if cfg.Apprise.Config != "" {
			if _, err := os.Stat(cfg.Apprise.Config); errors.Is(err, os.ErrNotExist) {
				return finish("missing_config")
			}
		}
		if urlsEnv != "" {
			value, ok := os.LookupEnv(urlsEnv)
			if !ok || value == "" {
				return finish("missing_urls")
			}
		}
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
	var env []string
	if route == "apprise" {
		args = []string{"--dry-run"}
		for _, option := range []struct{ flag, value string }{
			{"--config", cfg.Apprise.Config}, {"--tag", cfg.Apprise.Tag}, {"--input-format", cfg.Apprise.Format},
		} {
			if option.value != "" {
				args = append(args, option.flag, option.value)
			}
		}
		if urlsEnv != "" {
			value, _ := os.LookupEnv(urlsEnv)
			env = replaceEnvironment(os.Environ(), "APPRISE_URLS", value)
		} else if cfg.Apprise.Tag != "" {
			env = removeEnvironment(os.Environ(), "APPRISE_URLS")
		}
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if route == "herdr" {
		env = herdrEnv()
	}
	cmd.Env = env
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
