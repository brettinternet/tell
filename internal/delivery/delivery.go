// Package delivery executes one notification without retries or raw provider diagnostics.
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
	SchemaVersion int       `json:"schema_version"`
	Redactions    int       `json:"redactions"`
	Outcome       string    `json:"outcome"`
	Route         string    `json:"route"`
	Recipient     string    `json:"recipient,omitempty"`
	Intent        string    `json:"intent,omitempty"`
	Attempts      []Attempt `json:"attempts,omitempty"`
	Failure       *Failure  `json:"error,omitempty"`
}

type Attempt struct {
	Route   string `json:"route"`
	Label   string `json:"label"`
	Outcome string `json:"outcome"`
	Reason  string `json:"reason"`
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

func Notify(ctx context.Context, cfg config.Config, message Message, stderr io.Writer) *Result {
	ctx = childContext(ctx, cfg)
	steps, err := config.EffectiveNotifySteps(cfg)
	if err != nil {
		return NewResult(cfg.Notify.Via, "config", "invalid_config", "notification configuration is invalid")
	}
	if len(steps) == 0 {
		return NewResult(cfg.Notify.Via, "config", "invalid_config", "notification route is not configured")
	}
	if len(cfg.Notify.Steps) > 0 {
		for _, step := range steps {
			if step.Via == "command" && len(cfg.Notify.Command) == 0 {
				return NewResult(step.Via, "config", "invalid_config", "command route requires notify.command in personal config")
			}
		}
	}
	timeout, _ := time.ParseDuration(cfg.Timeout) // validated configuration
	if len(cfg.Notify.Steps) == 0 {
		message, redactions, reason := prepare(message, cfg.Secrets.Policy)
		if reason != "" {
			return NewResult(steps[0].Via, "config", reason, "notification rejected during message preparation")
		}
		if ctx.Err() != nil {
			result := NewResult(steps[0].Via, "config", "cancelled", "notification cancelled before delivery")
			result.Redactions = redactions
			return result
		}
		stepCfg := notifyStepConfig(cfg, steps[0])
		result := notifyStep(ctx, stepCfg, steps[0], message, stderr, timeout, false)
		result.Redactions = redactions
		result.Attempts = []Attempt{{Route: steps[0].Via, Label: config.NotifyStepLabel(steps[0]), Outcome: result.Outcome, Reason: resultReason(result)}}
		return result
	}
	budget, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	fallbackOn := cfg.Notify.FallbackOn
	if len(fallbackOn) == 0 {
		fallbackOn = []string{"not_sent"}
	}
	var attempts []Attempt
	var lastResult *Result
	var firstLabel, firstReason string
	stopAtBudget := func(step config.NotifyStep, label string) *Result {
		for _, attempt := range attempts {
			if attempt.Outcome != "not_sent" {
				if lastResult != nil {
					lastResult.Attempts = attempts
					return lastResult
				}
				break
			}
		}
		reason := "timeout_budget"
		if !errors.Is(budget.Err(), context.DeadlineExceeded) {
			reason = "cancelled"
		}
		result := NewResult(step.Via, "not_sent", reason, "notification cascade stopped before the next route")
		attempts = append(attempts, Attempt{Route: step.Via, Label: label, Outcome: result.Outcome, Reason: reason})
		result.Attempts = attempts
		return result
	}
	for i, step := range steps {
		label := config.NotifyStepLabel(step)
		if budget.Err() != nil {
			return stopAtBudget(step, label)
		}
		stepCfg := notifyStepConfig(cfg, step)
		stepMessage := message
		if i > 0 && cfg.Notify.FallbackNotice == "title" {
			stepMessage.Title = "Fallback from " + firstLabel + " (" + firstReason + "): " + stepMessage.Title
		}
		prepared, redactions, reason := prepare(stepMessage, cfg.Secrets.Policy)
		if reason != "" {
			result := NewResult(step.Via, "config", reason, "notification rejected during message preparation")
			result.Redactions = redactions
			attempts = append(attempts, Attempt{Route: step.Via, Label: label, Outcome: result.Outcome, Reason: reason})
			result.Attempts = attempts
			return result
		}
		if budget.Err() != nil {
			return stopAtBudget(step, label)
		}
		result := notifyStep(budget, stepCfg, step, prepared, stderr, timeout, true)
		lastResult = result
		result.Redactions = redactions
		attemptReason := ""
		if result.Failure != nil {
			attemptReason = result.Failure.Reason
		}
		attempts = append(attempts, Attempt{Route: step.Via, Label: label, Outcome: result.Outcome, Reason: attemptReason})
		if i == 0 {
			firstLabel, firstReason = label, attemptReason
		}
		result.Attempts = attempts
		if result.Outcome == "accepted" || !fallbackAllowed(fallbackOn, result.Outcome) || i == len(steps)-1 {
			return result
		}
	}
	return NewResult(cfg.Notify.Via, "not_sent", "timeout_budget", "notification cascade did not complete")
}

func notifyStepConfig(cfg config.Config, step config.NotifyStep) config.Config {
	cfg.Notify.Via = step.Via
	cfg.Notify.Steps = nil
	if step.Via == "apprise" {
		cfg.Apprise.Config, cfg.Apprise.Tag, cfg.Apprise.Format = step.Config, step.Tag, step.Format
	}
	if step.Via == "herdr" {
		cfg.Herdr.Session = step.Session
	}
	return cfg
}

func resultReason(result *Result) string {
	if result != nil && result.Failure != nil {
		return result.Failure.Reason
	}
	return ""
}

func fallbackAllowed(outcomes []string, outcome string) bool {
	for _, allowed := range outcomes {
		if allowed == outcome && outcome != "uncertain" {
			return true
		}
	}
	return false
}

func notifyStep(ctx context.Context, cfg config.Config, step config.NotifyStep, message Message, stderr io.Writer, timeout time.Duration, checkBeforeStart bool) *Result {
	route := step.Via
	if err := ctx.Err(); checkBeforeStart && err != nil {
		reason := "cancelled"
		if errors.Is(err, context.DeadlineExceeded) {
			reason = "timeout_budget"
		}
		return NewResult(route, "not_sent", reason, "notification route did not start")
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
	case "herdr":
		return notifyHerdr(ctx, timeout, cfg.Herdr.Session, message)
	case "desktop":
		return notifyDesktop(ctx, timeout, message, runtime.GOOS)
	case "apprise":
		if cfg.Apprise.Config != "" {
			if _, err := os.Stat(cfg.Apprise.Config); errors.Is(err, os.ErrNotExist) {
				return NewResult(route, "not_sent", "missing_config", "Apprise config file was not found")
			}
		}
		var env []string
		if step.UrlsEnv != "" {
			value, ok := os.LookupEnv(step.UrlsEnv)
			if !ok || value == "" {
				return NewResult(route, "not_sent", "missing_urls", "Apprise URL environment variable is unset or empty")
			}
			env = replaceEnvironment(os.Environ(), "APPRISE_URLS", value)
		} else if cfg.Apprise.Tag != "" {
			env = removeEnvironment(os.Environ(), "APPRISE_URLS")
		}
		args := []string{"--title", message.Title, "--body", message.Body, "--notification-type", message.Type}
		for _, option := range []struct{ flag, value string }{
			{"--config", cfg.Apprise.Config}, {"--tag", cfg.Apprise.Tag}, {"--input-format", cfg.Apprise.Format},
		} {
			if option.value != "" {
				args = append(args, option.flag, option.value)
			}
		}
		output := &probeOutput{}
		code, failure := process(ctx, timeout, route, "apprise", args, nil, env, output, output)
		if failure != nil {
			return failure
		}
		result := func(outcome, reason, message string) *Result {
			result := NewResult(route, outcome, reason, message)
			if result.Failure != nil {
				result.Failure.Message += appriseDiagnostic(code, output)
			}
			return result
		}
		// AppriseResultStatus (apprise/result.py): 0 success, 3 no match,
		// 4 partial, 5 timeout. CLI usage and unknown exits are failures.
		switch code {
		case 0:
			return NewResult(route, "accepted", "", "")
		case 3:
			return result("not_sent", "no_match", "Apprise matched no destination")
		case 4:
			return result("failed", "partial", "Apprise reported partial delivery")
		case 5:
			return result("uncertain", "timeout", "Apprise reported a timeout; delivery may have occurred")
		default:
			return result("failed", "provider_failure", "Apprise reported failure")
		}
	default:
		return NewResult(route, "not_sent", "unsupported_route", "notification route is not implemented")
	}
}

func removeEnvironment(env []string, name string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key != name {
			result = append(result, entry)
		}
	}
	return result
}

func replaceEnvironment(env []string, name, value string) []string {
	return append(removeEnvironment(env, name), name+"="+value)
}

// RunProcess bounds provider execution, supplies EOF on stdin, and discards both
// output streams. It returns provider exits separately from lifecycle failures.
func RunProcess(ctx context.Context, timeout time.Duration, route, name string, args []string) (int, *Result) {
	return runProcess(ctx, timeout, route, name, args, nil, nil)
}

// RunCommand executes trusted literal argv with caller-supplied stdin, environment,
// and working directory. Output is discarded through the shared process path.
func RunCommand(ctx context.Context, timeout time.Duration, route, name string, args []string, stdin io.Reader, env []string, dir string) (int, *Result) {
	return processDir(ctx, timeout, route, name, args, stdin, env, nil, nil, dir)
}

func runProcess(ctx context.Context, timeout time.Duration, route, name string, args []string, stdin io.Reader, env []string) (int, *Result) {
	return process(ctx, timeout, route, name, args, stdin, env, nil, nil)
}

func process(ctx context.Context, timeout time.Duration, route, name string, args []string, stdin io.Reader, env []string, stdout, stderr io.Writer) (int, *Result) {
	return processDir(ctx, timeout, route, name, args, stdin, env, stdout, stderr, "")
}

func processDir(ctx context.Context, timeout time.Duration, route, name string, args []string, stdin io.Reader, env []string, stdout, stderr io.Writer, dir string) (int, *Result) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.Stdin, cmd.Env, cmd.Dir = stdin, childEnv(ctx, env), dir
	// Bound stdin copying even if a descendant retains the pipe after exit.
	cmd.WaitDelay = 100 * time.Millisecond
	// Nil output streams use the null device; captured provider text is never printed.
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
	// A descendant can retain captured output pipes after the provider exits.
	// A deadline during pipe cleanup must not replace a known normal exit.
	if ctx.Err() != nil && (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) {
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
