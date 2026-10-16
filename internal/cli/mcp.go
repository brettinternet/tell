package cli

import (
	"context"
	"fmt"
	"os"

	tellmcp "github.com/brettinternet/tell/internal/mcp"
	"github.com/mattn/go-isatty"
	urfave "github.com/urfave/cli/v3"
)

func mcpCommand(version string) func(context.Context, *urfave.Command) error {
	return func(ctx context.Context, cmd *urfave.Command) error {
		if err := noArgs(cmd); err != nil {
			return err
		}
		if cmd.Bool("json") {
			return &failure{Code: "usage", Message: "mcp does not support --json; MCP protocol messages use stdout", usage: true}
		}
		if isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd()) {
			fmt.Fprintln(cmd.Writer, "tell mcp is a stdio MCP server. Start it from an MCP client:")
			fmt.Fprintln(cmd.Writer, "  claude mcp add tell -- tell mcp")
			fmt.Fprintln(cmd.Writer, "  codex mcp add tell -- tell mcp")
			fmt.Fprintln(cmd.Writer, "")
			fmt.Fprintln(cmd.Writer, "Other clients:")
			fmt.Fprintln(cmd.Writer, `{"mcpServers":{"tell":{"command":"tell","args":["mcp"]}}}`)
			return nil
		}
		values, err := input(cmd)
		if err != nil {
			return &failure{Code: "config", Message: "could not load MCP configuration"}
		}
		if values.File == nil {
			values.UserConfigDir, err = userConfigDir()
			if err != nil {
				return &failure{Code: "config", Message: "could not load MCP configuration"}
			}
		}
		server := tellmcp.NewServer(values, version, cmd.ErrWriter)
		return tellmcp.ServeStdio(ctx, server, os.Stdin, cmd.Writer)
	}
}
