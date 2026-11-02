package cli

import (
	"context"
	"fmt"

	"github.com/brettinternet/tell/internal/config"
	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

// exitStatus ends a command that already wrote its own output.
type exitStatus int

func (exitStatus) Error() string { return "exit status" }

func pushoverCommand() *urfave.Command {
	return &urfave.Command{
		Name: "pushover", Usage: "Check or cancel emergency Pushover alerts, one request each",
		Description: "With no subcommand, show help and credential readiness without contacting Pushover.\nTell never polls or waits; callers check receipts when they need to.\n\nExamples:\n  tell pushover\n  tell pushover status RECEIPT\n  tell pushover cancel --tag inc-0123abcd",
		Action:      pushoverInspect,
		Commands: []*urfave.Command{
			{Name: "status", Usage: "Report whether an emergency alert was acknowledged (exit 0) or not (exit 3)", ArgsUsage: "RECEIPT", Action: pushoverStatus},
			{Name: "cancel", Usage: "Stop repeating emergency alerts sent with --pushover-tag",
				Flags:  []urfave.Flag{&urfave.StringFlag{Name: "tag", Usage: "Pushover `tag` given to notify --pushover-tag"}},
				Action: pushoverCancel},
		},
	}
}

func pushoverInspect(_ context.Context, cmd *urfave.Command) error {
	cfg, err := load(cmd)
	if err != nil {
		return err
	}
	readiness := delivery.Readiness{Route: "pushover", Reason: delivery.PushoverReadiness(config.PushoverSettings(cfg))}
	readiness.Ready = readiness.Reason == "ready"
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"route": readiness.Route, "ready": readiness.Ready, "reason": readiness.Reason})
	}
	if err := urfave.ShowSubcommandHelp(cmd); err != nil {
		return err
	}
	style := commandStyle(cmd, cmd.Writer)
	if readiness.Ready {
		fmt.Fprintf(cmd.Writer, "\npushover: %s\n", style.success("ready"))
		return nil
	}
	fmt.Fprintf(cmd.Writer, "\npushover: not ready\n  %s\n", humanReason("pushover", readiness.Reason, ""))
	return nil
}

func pushoverStatus(ctx context.Context, cmd *urfave.Command) error {
	if cmd.NArg() != 1 || !delivery.ValidPushoverReceipt(cmd.Args().First()) {
		return &failure{Code: "usage", Message: "pushover status needs one receipt (letters and digits); run tell pushover status --help", usage: true}
	}
	cfg, err := loadSettings(cmd)
	if err != nil {
		return err
	}
	status, result := delivery.PushoverStatus(ctx, cfg, cmd.Args().First())
	if result != nil {
		return result
	}
	if cmd.Bool("json") {
		if err := emit(cmd, map[string]any{"acknowledged": status.Acknowledged, "expired": status.Expired}); err != nil {
			return err
		}
	} else {
		state := "not acknowledged"
		switch {
		case status.Acknowledged:
			state = "acknowledged"
		case status.Expired:
			state = "expired"
		}
		fmt.Fprintln(cmd.Writer, state)
	}
	if status.Acknowledged {
		return nil
	}
	return exitStatus(3)
}

func pushoverCancel(ctx context.Context, cmd *urfave.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	if !delivery.ValidPushoverTag(cmd.String("tag")) {
		return &failure{Code: "usage", Message: "pushover cancel needs --tag: letters, digits, '.', '_' or '-', starting with a letter or digit", usage: true}
	}
	cfg, err := loadSettings(cmd)
	if err != nil {
		return err
	}
	if result := delivery.PushoverCancel(ctx, cfg, cmd.String("tag")); result.ExitCode() != 0 {
		return result
	}
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"outcome": "accepted", "route": "pushover"})
	}
	return nil
}

// checkPushoverOptions rejects per-call Pushover options that no emergency
// pushover step would send, so a caller never assumes a tag it cannot cancel.
func checkPushoverOptions(cmd *urfave.Command, cfg config.Config) *delivery.Result {
	tag, callback := cmd.String("pushover-tag"), cmd.String("pushover-callback")
	if !cmd.IsSet("pushover-tag") && !cmd.IsSet("pushover-callback") {
		return nil
	}
	if cmd.IsSet("pushover-tag") && !delivery.ValidPushoverTag(tag) {
		return delivery.NewResult(cfg.Notify.Via, "usage", "invalid_input", "--pushover-tag must be letters, digits, '.', '_' or '-', starting with a letter or digit")
	}
	if cmd.IsSet("pushover-callback") && !delivery.ValidPushoverCallback(callback) {
		return delivery.NewResult(cfg.Notify.Via, "usage", "invalid_input", "--pushover-callback must be an http or https URL without credentials, at most 512 bytes")
	}
	steps, _ := config.EffectiveNotifySteps(cfg)
	for _, step := range steps {
		if step.Via == "pushover" && step.Emergency() {
			return nil
		}
	}
	return delivery.NewResult(cfg.Notify.Via, "usage", "invalid_input", "--pushover-tag and --pushover-callback need a pushover step with priority emergency")
}
