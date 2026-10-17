package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

func send(ctx context.Context, cmd *urfave.Command) error {
	// Return even accepted results to Run so every send has exactly one receipt.
	cfg, err := loadSettings(cmd)
	if err != nil {
		return delivery.NewResult("", "config", "invalid_config", err.Error())
	}
	intent := cmd.String("intent")
	var result *delivery.Result
	switch {
	case cmd.NArg() != 1:
		result = delivery.NewResult(cfg.Send.Via, "usage", "invalid_input", "send requires one message argument or - for stdin")
	case intent != "inform" && intent != "request":
		result = delivery.NewResult(cfg.Send.Via, "usage", "invalid_intent", fmt.Sprintf("invalid --intent %q; must be inform or request", usageValue(cmd, intent)))
	default:
		body := cmd.Args().First()
		if body == "-" {
			data, err := readMessage(ctx, cmd.Reader)
			if err != nil {
				reason := "input_io"
				if ctx.Err() != nil {
					reason = "cancelled"
				}
				result = delivery.NewResult(cfg.Send.Via, "config", reason, "could not read send input")
			}
			body = string(data)
		}
		if result == nil {
			result = delivery.Send(ctx, cfg, body, intent)
		}
	}
	if delivery.ValidRecipient(cfg.Send.Via, cfg.Send.To) {
		result.Recipient = cfg.Send.To
	}
	if intent == "inform" || intent == "request" {
		result.Intent = intent
	}
	return result
}

func renderSend(result *delivery.Result, jsonOutput bool, out, errOut io.Writer) {
	reason := ""
	if result.Failure != nil {
		reason = " reason=" + result.Failure.Reason
		if !jsonOutput && (result.Outcome == "usage" || result.Outcome == "config") {
			reason += ": " + result.Failure.Message
		}
	}
	fmt.Fprintf(errOut, "tell send: route=%q recipient=%q intent=%q outcome=%s%s\n", result.Route, result.Recipient, result.Intent, result.Outcome, reason)
	if jsonOutput {
		// A failed output stream cannot change the dispatch outcome or invite retry.
		_ = json.NewEncoder(out).Encode(result)
	}
}
