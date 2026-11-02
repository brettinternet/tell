package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

type targetsReport struct {
	SchemaVersion int               `json:"schema_version"`
	Route         string            `json:"route"`
	Targets       []delivery.Target `json:"targets"`
	failure       *delivery.Result
}

func (*targetsReport) Error() string { return "recipient listing" }

func targets(ctx context.Context, cmd *urfave.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	cfg, err := loadSettings(cmd)
	if err != nil {
		return err
	}
	list, failure := delivery.Targets(ctx, cfg)
	return &targetsReport{SchemaVersion: 1, Route: cfg.Send.Via, Targets: list, failure: failure}
}

func (r *targetsReport) render(jsonOutput bool, out, errOut io.Writer, style outputStyle) int {
	if r.failure != nil {
		if jsonOutput {
			_ = json.NewEncoder(out).Encode(r.failure)
		} else {
			fmt.Fprintf(errOut, "%s %s\n", style.failure("tell targets:"), humanReason("send/"+r.failure.Route, r.failure.Failure.Reason, r.failure.Failure.Message))
		}
		return r.failure.ExitCode()
	}
	var err error
	switch {
	case jsonOutput:
		err = json.NewEncoder(out).Encode(r)
	case len(r.Targets) == 0:
		_, err = fmt.Fprintf(out, "No %s recipients found\n", r.Route)
	default:
		var rendered strings.Builder
		table := tabwriter.NewWriter(&rendered, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "TO\tKIND\tSTATUS\tREADY")
		for _, target := range r.Targets {
			ready := "no"
			if target.Ready {
				ready = "yes"
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", target.To, target.Kind, target.Status, ready)
		}
		_ = table.Flush()
		// Style after alignment so escape sequences cannot widen cells.
		header, rows, _ := strings.Cut(rendered.String(), "\n")
		_, err = fmt.Fprintf(out, "%s\n%s", style.bold(header), rows)
	}
	if err != nil {
		fmt.Fprintf(errOut, "%s could not write recipient listing\n", style.failure("tell:"))
		return 1
	}
	return 0
}
