package cli

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

func notify(ctx context.Context, cmd *urfave.Command) error {
	cfg, err := loadSettings(cmd)
	if err != nil {
		return delivery.NewResult("", "config", "invalid_config", "could not load notification configuration; run tell config check")
	}
	route := cfg.Notify.Via
	if cmd.NArg() != 1 {
		return delivery.NewResult(route, "usage", "invalid_input", "notify requires one message argument or - for stdin")
	}
	kind := cmd.String("type")
	switch kind {
	case "info", "success", "warning", "failure":
	default:
		return delivery.NewResult(route, "usage", "invalid_type", "notification type must be info, success, warning, or failure")
	}
	body := cmd.Args().First()
	if body == "-" {
		data, err := readMessage(ctx, cmd.Reader)
		if ctx.Err() != nil {
			return delivery.NewResult(route, "config", "cancelled", "notification cancelled before delivery")
		}
		if err != nil {
			return delivery.NewResult(route, "config", "input_io", "could not read notification input")
		}
		body = string(data)
	}
	if strings.TrimSpace(body) == "" {
		return delivery.NewResult(route, "config", "empty_message", "notification message must not be empty")
	}
	title := cmd.String("title")
	if !cmd.IsSet("title") {
		title = filepath.Base(cfg.Project)
	}
	result := delivery.Notify(ctx, cfg, delivery.Message{Title: title, Body: body, Type: kind}, cmd.ErrWriter)
	if result.ExitCode() != 0 {
		return result
	}
	if cmd.Bool("json") {
		if err := emit(cmd, map[string]any{"outcome": result.Outcome, "route": result.Route}); err != nil {
			return &resultWriteError{}
		}
	}
	return nil
}

// A blocked stdin must not prevent the process boundary from honoring signals.
// The reader belongs to the caller; do not close it. On cancellation the buffered
// channel lets the read finish independently if the caller later closes input.
func readMessage(ctx context.Context, reader io.Reader) ([]byte, error) {
	type inputResult struct {
		data []byte
		err  error
	}
	done := make(chan inputResult, 1)
	go func() { data, err := io.ReadAll(reader); done <- inputResult{data, err} }()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-done:
		return result.data, result.err
	}
}

type resultWriteError struct{}

func (*resultWriteError) Error() string {
	return "notification accepted but result output failed; do not resend blindly"
}

// Locate the command without mistaking a flag value or message for a command.
func notifyRequested(root *urfave.Command, args []string) bool {
	valueFlags := map[string]bool{}
	for _, flag := range root.Flags {
		if _, boolean := flag.(*urfave.BoolFlag); !boolean {
			for _, name := range flag.Names() {
				valueFlags[name] = true
			}
		}
	}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return i+1 < len(args) && args[i+1] == "notify"
		}
		if !strings.HasPrefix(arg, "-") {
			return arg == "notify"
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if valueFlags[name] && !hasValue {
			i++
		}
	}
	return false
}
