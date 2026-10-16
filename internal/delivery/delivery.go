// Package delivery executes one notification without retries or provider diagnostics.
package delivery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

// Result describes dispatch, not human acknowledgment. It never contains input text.
type Result struct {
	SchemaVersion int      `json:"schema_version"`
	Redactions    int      `json:"redactions"`
	Outcome       string   `json:"outcome"`
	Route         string   `json:"route"`
	Failure       *Failure `json:"error,omitempty"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

func NewResult(route, outcome, reason, message string) *Result {
	r := &Result{SchemaVersion: 1, Route: route, Outcome: outcome}
	if outcome != "accepted" {
		r.Failure = &Failure{Code: outcome, Message: message, Reason: reason}
	}
	return r
}

func (r *Result) Error() string {
	if r.Failure != nil {
		return r.Failure.Message
	}
	return r.Outcome
}
func (r *Result) ExitCode() int {
	switch r.Outcome {
	case "accepted":
		return 0
	case "usage":
		return 2
	case "not_sent":
		return 3
	case "failed":
		return 4
	case "uncertain":
		return 5
	default:
		return 1
	}
}

type Message struct{ Title, Body, Type string }

func Notify(ctx context.Context, cfg config.Config, message Message, stderr io.Writer) (result *Result) {
	route := cfg.Notify.Via
	var reason string
	var redactions int
	defer func() { result.Redactions = redactions }()
	message, redactions, reason = prepare(message, cfg.Secrets.Policy)
	if reason != "" {
		return NewResult(route, "config", reason, "notification rejected during message preparation")
	}
	if ctx.Err() != nil {
		return NewResult(route, "config", "cancelled", "notification cancelled before delivery")
	}
	switch route {
	case "stderr":
		if _, err := fmt.Fprintln(stderr, message.Body); err != nil {
			return NewResult(route, "failed", "io", "could not write notification to stderr")
		}
		return NewResult(route, "accepted", "", "")
	case "command":
		if len(cfg.Notify.Command) == 0 {
			return NewResult(route, "config", "invalid_config", "command route requires notify.command in personal config")
		}
		timeout, _ := time.ParseDuration(cfg.Timeout)
		env := append(os.Environ(), "TELL_TITLE="+message.Title, "TELL_TYPE="+message.Type, "TELL_PROJECT="+cfg.Project)
		code, failure := runProcess(ctx, timeout, route, cfg.Notify.Command[0], cfg.Notify.Command[1:], strings.NewReader(message.Body), env)
		if failure != nil {
			return failure
		}
		switch code {
		case 0:
			return NewResult(route, "accepted", "", "")
		case 3:
			return NewResult(route, "not_sent", "no_match", "command reported no delivery")
		default:
			return NewResult(route, "failed", "provider_failure", "command reported failure")
		}
	case "desktop":
		timeout, _ := time.ParseDuration(cfg.Timeout)
		return notifyDesktop(ctx, timeout, message, runtime.GOOS)
	case "apprise":
		args := []string{"--title", message.Title, "--body", message.Body, "--notification-type", message.Type}
		for _, option := range []struct{ flag, value string }{
			{"--config", cfg.Apprise.Config}, {"--tag", cfg.Apprise.Tag}, {"--input-format", cfg.Apprise.Format},
		} {
			if option.value != "" {
				args = append(args, option.flag, option.value)
			}
		}
		timeout, _ := time.ParseDuration(cfg.Timeout) // validated configuration
		code, failure := RunProcess(ctx, timeout, route, "apprise", args)
		if failure != nil {
			return failure
		}
		// AppriseResultStatus (apprise/result.py): 0 success, 3 no match,
		// 4 partial, 5 timeout. CLI usage and unknown exits are failures.
		switch code {
		case 0:
			return NewResult(route, "accepted", "", "")
		case 3:
			return NewResult(route, "not_sent", "no_match", "Apprise matched no destination")
		case 4:
			return NewResult(route, "failed", "partial", "Apprise reported partial delivery")
		case 5:
			return NewResult(route, "uncertain", "timeout", "Apprise reported a timeout; delivery may have occurred")
		default:
			return NewResult(route, "failed", "provider_failure", "Apprise reported failure")
		}
	default:
		return NewResult(route, "not_sent", "unsupported_route", "notification route is not implemented")
	}
}

// RunProcess bounds provider execution, supplies EOF on stdin, and discards both
// output streams. It returns provider exits separately from lifecycle failures.
func RunProcess(ctx context.Context, timeout time.Duration, route, name string, args []string) (int, *Result) {
	return runProcess(ctx, timeout, route, name, args, nil, nil)
}

func runProcess(ctx context.Context, timeout time.Duration, route, name string, args []string, stdin io.Reader, env []string) (int, *Result) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin, cmd.Env = stdin, env
	// Bound stdin copying even if a descendant retains the pipe after exit.
	cmd.WaitDelay = 100 * time.Millisecond
	// Nil output streams use the null device; provider text is never captured.
	if err := cmd.Start(); err != nil {
		if ctx.Err() != nil {
			return 0, NewResult(route, "config", "cancelled", "notification cancelled before provider start")
		}
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return 0, NewResult(route, "not_sent", "missing_executable", "provider executable was not found")
		}
		return 0, NewResult(route, "not_sent", "start_failed", "provider executable could not start")
	}
	err := cmd.Wait()
	if ctx.Err() != nil {
		reason := "cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = "timeout"
		}
		return 0, NewResult(route, "uncertain", reason, "provider interrupted after start; delivery may have occurred")
	}
	// ErrWaitDelay means the child exited successfully but a descendant held
	// the stdin pipe open. Pipe cleanup must not change its accepted exit.
	if err == nil || errors.Is(err, exec.ErrWaitDelay) {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 0, NewResult(route, "failed", "provider_failure", "could not collect provider result")
}
