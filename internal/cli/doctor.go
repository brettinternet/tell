package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

type doctorReport struct {
	SchemaVersion int                  `json:"schema_version"`
	Selected      string               `json:"selected"`
	Ready         bool                 `json:"ready"`
	Routes        []delivery.Readiness `json:"routes"`
}

func (*doctorReport) Error() string { return "route readiness report" }

func doctor(ctx context.Context, cmd *urfave.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	cfg, err := loadSettings(cmd)
	if err != nil {
		return &failure{Code: "config", Message: "could not load configuration; run tell config check"}
	}
	routes := delivery.Doctor(ctx, cfg)
	return &doctorReport{SchemaVersion: 1, Selected: cfg.Notify.Via, Ready: routes[0].Ready, Routes: routes}
}

func (r *doctorReport) render(root *urfave.Command, args []string, out, errOut io.Writer) int {
	var err error
	if jsonRequested(root, args) {
		err = json.NewEncoder(out).Encode(r)
	} else {
		for _, route := range r.Routes {
			selected := ""
			if route.Route == r.Selected {
				selected = " (selected)"
			}
			count := ""
			if route.Services != nil {
				count = fmt.Sprintf("; services: %d", *route.Services)
			}
			if _, err = fmt.Fprintf(out, "%s%s: %s%s\n", route.Route, selected, route.Reason, count); err != nil {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintln(errOut, "tell: could not write readiness report")
		return 1
	}
	if !r.Ready {
		return 1
	}
	return 0
}
