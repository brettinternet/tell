package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/brettinternet/tell/internal/listen"
	urfave "github.com/urfave/cli/v3"
)

// Runtime diagnostics have already passed through the bounded listener writer.
// Never retry a write on a stalled stderr from the generic CLI error renderer.
type listenerFailure struct{ error }

func listenCommand() *urfave.Command {
	return &urfave.Command{
		Name: "listen", Usage: "Inspect configured sources or explicitly capture events", ArgsUsage: "[slack|github...]",
		Description: "With no source, inspect and exit without connecting. Explicit source selection starts a foreground listener.\n--print exports sensitive envelopes; --dry-run validates matching actions without delivery.\nRun actions use literal argv, receive the envelope on stdin, and have stdout/stderr discarded. Native notify/send actions use single-pass scalar envelope templates.\n\nExamples:\n  tell listen\n  tell --json listen\n  tell listen slack\n  tell listen github\n  tell listen slack github\n  tell listen --dry-run github\n  tell listen --print github\n  tell listen --dry-run --all",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "all", Usage: "Start every configured source (cannot accompany source names)"},
			&urfave.BoolFlag{Name: "print", Usage: "Export sensitive NDJSON events without matching or running actions"},
			&urfave.BoolFlag{Name: "dry-run", Usage: "Report body-free route matches without running actions"},
		},
		ShellComplete: func(_ context.Context, cmd *urfave.Command) { fmt.Fprintln(cmd.Writer, "slack github") },
		Action:        listenAction,
	}
}

func listenAction(ctx context.Context, cmd *urfave.Command) error {
	return runListen(ctx, cmd, listen.Capture)
}

func runListen(ctx context.Context, cmd *urfave.Command, capture func(context.Context, listen.CaptureOptions) error) error {
	names := cmd.Args().Slice()
	usage := func(message string) error { return &failure{Code: "usage", Message: message, usage: true} }
	if cmd.Bool("print") && cmd.Bool("dry-run") {
		return usage("--print and --dry-run cannot be combined")
	}
	if len(names) > 2 {
		return usage("select each source once; run tell listen --help")
	}
	seen := map[string]bool{}
	for _, name := range names {
		if name != "slack" && name != "github" {
			return usage("select slack or github; run tell listen --help")
		}
		if seen[name] {
			return usage("select each source once; run tell listen --help")
		}
		seen[name] = true
	}
	if cmd.Bool("all") && len(names) > 0 {
		return usage("--all cannot accompany source names")
	}
	live := len(names) > 0 || cmd.Bool("all")
	preview := cmd.Bool("print") || cmd.Bool("dry-run")
	if preview && !live {
		return usage("--print and --dry-run require a source name or --all")
	}
	if live && cmd.Bool("json") {
		return usage("--json is for inspection only; use --print or --dry-run for live capture")
	}

	cfg, err := loadSettings(cmd)
	if err != nil {
		return err
	}
	if !live {
		sources := []string{}
		if cfg.Listen.Slack != nil {
			sources = append(sources, "slack")
		}
		if cfg.Listen.GitHub != nil {
			sources = append(sources, "github")
		}
		routes := make([]string, 0, len(cfg.Listen.Routes))
		for _, route := range cfg.Listen.Routes {
			routes = append(routes, route.Name)
		}
		if cmd.Bool("json") {
			return emit(cmd, map[string]any{"sources": sources, "routes": routes, "listen": cfg.Listen, "origins": cfg.Origins})
		}
		if len(sources) == 0 {
			_, err = fmt.Fprintln(cmd.Writer, "Listen: no sources configured")
		} else {
			for _, source := range sources {
				switch source {
				case "slack":
					_, err = fmt.Fprintf(cmd.Writer, "Listen: slack (credential variable %s)\n", cfg.SlackTokenEnv())
				case "github":
					_, err = fmt.Fprintf(cmd.Writer, "Listen: github (credential variable %s; repositories %s; HTTP %s)\n", cfg.GitHubSecretEnv(), cfg.Values()["listen.github.repository_ids"], cfg.HTTPAddr())
				}
				if err != nil {
					return err
				}
			}
		}
		if err != nil {
			return err
		}
		if len(routes) == 0 {
			_, err = fmt.Fprintln(cmd.Writer, "Routes: none")
		} else {
			_, err = fmt.Fprintf(cmd.Writer, "Routes: %s\n", joinNames(routes))
		}
		return err
	}

	selected := names
	if cmd.Bool("all") {
		selected = make([]string, 0, 2)
		if cfg.Listen.Slack != nil {
			selected = append(selected, "slack")
		}
		if cfg.Listen.GitHub != nil {
			selected = append(selected, "github")
		}
		if len(selected) == 0 {
			return errors.New("no configured listen sources")
		}
	}
	for _, source := range selected {
		switch source {
		case "slack":
			if cfg.Listen.Slack == nil {
				return errors.New("slack is not configured; set listen.slack.app_token_env in personal config")
			}
		case "github":
			if cfg.Listen.GitHub == nil {
				return errors.New("github is not configured; set listen.github in personal config")
			}
		}
	}
	if !cmd.Bool("print") {
		for _, source := range selected {
			found := false
			for _, route := range cfg.Listen.Routes {
				if route.Source == source {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no routes configured for %s; add a personal listen.routes entry", source)
			}
		}
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	sources := make([]listen.SourceOptions, 0, len(selected))
	for _, source := range selected {
		switch source {
		case "slack":
			token := os.Getenv(cfg.SlackTokenEnv())
			if token == "" {
				return errors.New("selected Slack credential is missing or empty; set the configured app_token_env variable")
			}
			sources = append(sources, listen.SourceOptions{Name: "slack", Token: token, Timeout: timeout})
		case "github":
			secret := os.Getenv(cfg.GitHubSecretEnv())
			if secret == "" {
				return errors.New("selected GitHub credential is missing or empty; set the configured secret_env variable")
			}
			sources = append(sources, listen.SourceOptions{Name: "github", Secret: secret, Addr: cfg.HTTPAddr(), RepositoryIDs: append([]int64(nil), cfg.Listen.GitHub.RepositoryIDs...), Timeout: timeout})
		}
	}
	options := listen.CaptureOptions{Sources: sources, Out: cmd.Writer, Err: cmd.ErrWriter}
	var executor *listen.Executor
	if cmd.Bool("dry-run") {
		options.OnEvent = func(event listen.Envelope, report func(string, string, string), _ io.Writer) {
			matched := listen.Matches(cfg.Listen.Routes, event)
			if len(matched) == 0 {
				report("", "no_match", "")
				return
			}
			for _, route := range matched {
				outcome, reason := listen.ValidateAction(ctx, cfg, route, event)
				report(route.Name, outcome, reason)
			}
		}
	} else if !cmd.Bool("print") {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		executor = listen.NewExecutor(cfg, cwd)
		options.OnEvent = func(event listen.Envelope, report func(string, string, string), deliveryWriter io.Writer) {
			matched := listen.Matches(cfg.Listen.Routes, event)
			if len(matched) == 0 {
				report("", "no_match", "")
				return
			}
			for _, route := range matched {
				if !executor.SubmitWithWriter(route, event, report, deliveryWriter) {
					report(route.Name, "not_sent", "busy")
				}
			}
		}
	}
	if executor == nil {
		if err := capture(ctx, options); err != nil {
			return &listenerFailure{err}
		}
		return nil
	}

	shutdown := make(chan struct{})
	finished := make(chan struct{})
	var finishOnce sync.Once
	finishActions := func() {
		finishOnce.Do(func() {
			close(shutdown)
			executor.Stop()
			executor.Wait()
			close(finished)
		})
	}
	options.OnStop = finishActions
	go func() {
		select {
		case <-ctx.Done():
		case <-shutdown:
		case <-finished:
			return
		}
		executor.Stop()
		select {
		case <-shutdownSignal(ctx):
			executor.CancelRunning()
		case <-finished:
		}
	}()
	err = capture(ctx, options)
	finishActions()
	if err != nil {
		return &listenerFailure{err}
	}
	return nil
}

func joinNames(names []string) string {
	result := ""
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}
