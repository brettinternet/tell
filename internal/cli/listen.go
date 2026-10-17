package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brettinternet/tell/internal/listen"
	urfave "github.com/urfave/cli/v3"
)

// Runtime diagnostics have already passed through the bounded listener writer.
// Never retry a write on a stalled stderr from the generic CLI error renderer.
type listenerFailure struct{ error }

func listenCommand() *urfave.Command {
	return &urfave.Command{
		Name: "listen", Usage: "Inspect sources, or explicitly capture Slack events", ArgsUsage: "[slack]",
		Description: "With no source, inspect and exit without connecting. Only --print capture is supported; routing is not yet available.\nPrint exports sensitive event payloads as NDJSON, not sanitized diagnostics.\n\nExamples:\n  tell listen\n  tell --json listen\n  tell listen --print slack\n  tell listen --print --all",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{Name: "all", Usage: "Start every configured source (cannot accompany source names)"},
			&urfave.BoolFlag{Name: "print", Usage: "Export sensitive NDJSON events without running actions"},
		},
		ShellComplete: func(_ context.Context, cmd *urfave.Command) { fmt.Fprintln(cmd.Writer, "slack") },
		Action:        listenAction,
	}
}
func listenAction(ctx context.Context, cmd *urfave.Command) error {
	return runListen(ctx, cmd, listen.Slack)
}
func runListen(ctx context.Context, cmd *urfave.Command, capture func(context.Context, listen.SlackOptions) error) error {
	names := cmd.Args().Slice()
	live := len(names) > 0 || cmd.Bool("all")
	usage := func(message string) error { return &failure{Code: "usage", Message: message, usage: true} }
	if len(names) > 1 || (len(names) == 1 && names[0] != "slack") {
		return usage("select slack once; run tell listen --help")
	}
	if cmd.Bool("all") && len(names) > 0 {
		return usage("--all cannot accompany source names")
	}
	if cmd.Bool("print") && !live {
		return usage("--print requires slack or --all")
	}
	if live && cmd.Bool("json") {
		return usage("--json is for inspection only; use --print for live capture")
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
		if cmd.Bool("json") {
			return emit(cmd, map[string]any{"sources": sources, "routes": []string{}, "listen": cfg.Listen, "origins": cfg.Origins})
		}
		if len(sources) == 0 {
			_, err = fmt.Fprintln(cmd.Writer, "Listen: no sources configured")
		} else {
			_, err = fmt.Fprintf(cmd.Writer, "Listen: slack (credential variable %s)\nRoutes: none; use tell listen --print slack to export sensitive events\n", cfg.SlackTokenEnv())
		}
		return err
	}
	if cfg.Listen.Slack == nil {
		return errors.New("slack is not configured; set listen.slack.app_token_env in personal config")
	}
	if !cmd.Bool("print") {
		return errors.New("no routes configured; routing is not yet available, use tell listen --print slack for sensitive event export")
	}
	token := os.Getenv(cfg.SlackTokenEnv())
	if token == "" {
		return errors.New("selected Slack credential is missing or empty; set the configured app_token_env variable")
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	if err := capture(ctx, listen.SlackOptions{Token: token, Timeout: timeout, Out: cmd.Writer, Err: cmd.ErrWriter}); err != nil {
		return &listenerFailure{err}
	}
	return nil
}
