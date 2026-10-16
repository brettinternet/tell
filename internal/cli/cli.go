// Package cli adapts configuration operations to human and machine interfaces.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/brettinternet/tell/internal/config"
	"github.com/brettinternet/tell/internal/delivery"
	urfave "github.com/urfave/cli/v3"
)

type settingSpec struct{ key, flag, env, usage string }

var settings = []settingSpec{
	{"timeout", "timeout", "TELL_TIMEOUT", "Provider execution timeout (positive duration)"},
	{"notify.via", "via", "TELL_NOTIFY_VIA", "Notification destination: stderr, desktop, apprise, command, herdr"},
	{"apprise.config", "apprise-config", "TELL_APPRISE_CONFIG", "Local Apprise config path; Tell does not read its contents"},
	{"apprise.tag", "apprise-tag", "TELL_APPRISE_TAG", "Apprise tag expression"},
	{"apprise.format", "apprise-format", "TELL_APPRISE_FORMAT", "Apprise body format: text, markdown, html"},
	{"herdr.session", "herdr-session", "TELL_HERDR_SESSION", "Explicit Herdr session"},
	{"send.via", "send-via", "TELL_SEND_VIA", "Agent transport: hcom or herdr"},
	{"send.to", "to", "TELL_SEND_TO", "One exact agent recipient (no @ prefix)"},
	{"send.from", "from", "TELL_SEND_FROM", "Automated sender identity (default: tell-automation)"},
}

// NewRootCommand constructs a fresh tree without reading files or executing tools.
func NewRootCommand(build config.BuildOpts, out, errOut io.Writer) *urfave.Command {
	root := &urfave.Command{
		Name: "tell", Usage: "Delivery plumbing for humans and agents",
		Description: "Inspect settings safely, or notify via stderr, desktop, Apprise, Herdr, or a personal command. Provider acceptance is not human acknowledgment. With no command, inspect only.",
		Version:     fmt.Sprintf("%s (commit %s, built %s)", build.Version, build.Commit, build.BuildTime),
		Writer:      out, ErrWriter: errOut, EnableShellCompletion: true,
		ExitErrHandler: func(context.Context, *urfave.Command, error) {},
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "project", Aliases: []string{"C"}, Usage: "Select exactly this project directory", Sources: urfave.EnvVars("TELL_PROJECT")},
			&urfave.StringFlag{Name: "config", Usage: "Replace all automatic config files with FILE", Sources: urfave.EnvVars("TELL_CONFIG")},
			&urfave.BoolFlag{Name: "json", Usage: "Print one structured JSON result"},
			&urfave.BoolFlag{Name: "mcp-agent-targets", Usage: "Allow MCP agents to override route and recipient selectors [$TELL_MCP_AGENT_TARGETS]"},
		},
		Action: showConfig,
		Commands: []*urfave.Command{
			{Name: "send", Usage: "Send to an existing agent; dispatch is not completion", ArgsUsage: "MESSAGE | -", Flags: []urfave.Flag{
				&urfave.StringFlag{Name: "intent", Value: "inform", Usage: "Message intent: inform or request"},
			}, Action: send},
			{Name: "doctor", Usage: "Check configured routes without sending or starting services", Action: doctor},
			{Name: "mcp", Usage: "Serve notify, send, and doctor as MCP tools over stdio", Action: mcpCommand(build.Version)},
			{Name: "notify", Usage: "Notify via the selected route; acceptance is not human acknowledgment", ArgsUsage: "MESSAGE | -", Flags: []urfave.Flag{
				&urfave.StringFlag{Name: "title", Usage: "Notification title (default: project directory name)"},
				&urfave.StringFlag{Name: "type", Value: "info", Usage: "Notification type: info, success, warning, failure"},
			}, Action: notify},
			{Name: "config", Usage: "Inspect effective settings and origins without sending anything", Action: showConfig, Commands: []*urfave.Command{
				{Name: "show", Usage: "Show effective settings and origins", Action: showConfig},
				{Name: "check", Usage: "Validate settings without invoking providers", Action: checkConfig},
			}},
			{Name: "init", Usage: "Create tell.yaml without overwriting; never sends messages", Flags: []urfave.Flag{
				&urfave.BoolFlag{Name: "local", Usage: "Create private .tell.yaml instead; add it to .gitignore"},
				&urfave.BoolFlag{Name: "user", Usage: "Create personal user config instead"},
			}, Action: initialize},
		},
	}
	for _, spec := range settings {
		// Resolve these env vars in input(), keeping exact flag/env provenance
		// instead of losing it in the CLI framework's combined value source.
		root.Flags = append(root.Flags, &urfave.StringFlag{Name: spec.flag, Usage: spec.usage + " [$" + spec.env + "]"})
	}
	_ = root.Walk(func(cmd *urfave.Command) error {
		cmd.HideHelpCommand = len(cmd.Commands) == 0
		cmd.OnUsageError = func(_ context.Context, _ *urfave.Command, err error, _ bool) error {
			return &failure{Code: "usage", Message: err.Error(), usage: true}
		}
		return nil
	})
	return root
}

type failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	usage   bool
}

func (err *failure) Error() string { return err.Message }

// Run renders errors without exiting the caller. JSON actions produce one object;
// help, version, and shell completion retain their standard text formats.
func Run(ctx context.Context, args []string, build config.BuildOpts, out, errOut io.Writer) int {
	root := NewRootCommand(build, out, errOut)
	if err := root.Run(ctx, args); err != nil {
		var report *doctorReport
		if errors.As(err, &report) {
			return report.render(root, args, out, errOut)
		}
		var outputError *resultWriteError
		if errors.As(err, &outputError) {
			// Delivery was accepted. A broken result stream must not invite a resend
			// by reporting usage/config (both promise nothing was sent).
			fmt.Fprintf(errOut, "tell: %s\n", outputError.Error())
			return 0
		}
		var result *delivery.Result
		if commandRequested(root, args, "send") {
			if !errors.As(err, &result) {
				result = delivery.NewResult("", "usage", "invalid_invocation", "invalid send invocation; run tell send --help")
			}
			renderSend(result, jsonRequested(root, args), out, errOut)
			return result.ExitCode()
		}
		if !errors.As(err, &result) && notifyRequested(root, args) {
			result = delivery.NewResult("", "usage", "invalid_invocation", "invalid notify invocation; run tell notify --help")
		}
		if result != nil {
			if jsonRequested(root, args) {
				_ = json.NewEncoder(out).Encode(result)
			} else {
				fmt.Fprintf(errOut, "tell: %s\n", result.Error())
			}
			return result.ExitCode()
		}
		var issue *failure
		if !errors.As(err, &issue) {
			issue = &failure{Code: "error", Message: err.Error()}
		}
		if jsonRequested(root, args) {
			_ = json.NewEncoder(out).Encode(map[string]any{"schema_version": 1, "error": issue})
		} else {
			fmt.Fprintf(errOut, "tell: %s\n", issue.Message)
		}
		if issue.usage {
			return 2
		}
		return 1
	}
	return 0
}

func input(cmd *urfave.Command) (config.Input, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return config.Input{}, err
	}
	values := config.Input{CWD: cwd, Project: selector(cmd, "project"), File: selector(cmd, "config"), Overrides: map[string]config.Value{}}
	for _, spec := range settings {
		if cmd.IsSet(spec.flag) {
			values.Overrides[spec.key] = config.Value{Text: cmd.String(spec.flag), Source: "flag:--" + spec.flag}
		} else if value, exists := os.LookupEnv(spec.env); exists {
			values.Overrides[spec.key] = config.Value{Text: value, Source: "env:" + spec.env}
		}
	}
	if cmd.IsSet("mcp-agent-targets") {
		values.Overrides["mcp.agent_targets"] = config.Value{Text: strconv.FormatBool(cmd.Bool("mcp-agent-targets")), Source: "flag:--mcp-agent-targets"}
	} else if value, exists := os.LookupEnv("TELL_MCP_AGENT_TARGETS"); exists {
		values.Overrides["mcp.agent_targets"] = config.Value{Text: value, Source: "env:TELL_MCP_AGENT_TARGETS"}
	}
	return values, nil
}

// userConfigDir is needed only for automatic file sources, so an explicit
// --config works in service environments without HOME.
func userConfigDir() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	if !filepath.IsAbs(dir) {
		return "", errors.New("XDG_CONFIG_HOME must be absolute")
	}
	return dir, nil
}

func selector(cmd *urfave.Command, name string) *string {
	if !cmd.IsSet(name) {
		return nil
	}
	value := cmd.String(name)
	return &value
}

func noArgs(cmd *urfave.Command) error {
	if cmd.NArg() != 0 {
		return &failure{Code: "usage", Message: fmt.Sprintf("unexpected argument %q; run tell --help", cmd.Args().First()), usage: true}
	}
	return nil
}

func load(cmd *urfave.Command) (config.Config, error) {
	if err := noArgs(cmd); err != nil {
		return config.Config{}, err
	}
	return loadSettings(cmd)
}

func loadSettings(cmd *urfave.Command) (config.Config, error) {
	values, err := input(cmd)
	if err == nil && values.File == nil {
		values.UserConfigDir, err = userConfigDir()
	}
	if err == nil {
		var cfg config.Config
		cfg, err = config.Load(values)
		if err == nil {
			return cfg, nil
		}
	}
	return config.Config{}, &failure{Code: "config", Message: err.Error()}
}

func showConfig(_ context.Context, cmd *urfave.Command) error {
	cfg, err := load(cmd)
	if err != nil {
		return err
	}
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"config": cfg})
	}
	fmt.Fprintf(cmd.Writer, "Project: %q\n", cfg.Project)
	values := cfg.Values()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		fmt.Fprintf(cmd.Writer, "%s: %q (%q)\n", key, values[key], cfg.Origins[key])
	}
	fmt.Fprintln(cmd.Writer, "Files (low → high precedence):")
	if len(cfg.Sources) == 0 {
		fmt.Fprintln(cmd.Writer, "  none")
	}
	for _, source := range cfg.Sources {
		fmt.Fprintf(cmd.Writer, "  %q\n", source)
	}
	fmt.Fprintln(cmd.Writer, "Inspection only; notify delivers via the selected route. Provider acceptance is not human acknowledgment.")
	return nil
}

func checkConfig(_ context.Context, cmd *urfave.Command) error {
	cfg, err := load(cmd)
	if err != nil {
		return err
	}
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"valid": true, "config": cfg})
	}
	fmt.Fprintf(cmd.Writer, "Configuration valid · %q\n", cfg.Project)
	return nil
}

func initialize(_ context.Context, cmd *urfave.Command) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	values, err := input(cmd)
	if err != nil {
		return err
	}
	if values.File != nil || len(values.Overrides) != 0 {
		return &failure{Code: "usage", Message: "init does not apply config or setting overrides; edit the generated file instead", usage: true}
	}
	if cmd.Bool("user") && (cmd.Bool("local") || values.Project != nil) {
		return &failure{Code: "usage", Message: "--user cannot be combined with --local or --project", usage: true}
	}
	var path string
	var created bool
	if cmd.Bool("user") {
		values.UserConfigDir, err = userConfigDir()
		if err == nil {
			path, created, err = config.InitUser(values)
		}
	} else {
		path, created, err = config.Init(values, cmd.Bool("local"))
	}
	if err != nil {
		return err
	}
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"path": path, "created": created})
	}
	if created {
		fmt.Fprintf(cmd.Writer, "Created %q\n", path)
	} else {
		fmt.Fprintf(cmd.Writer, "Kept existing %q (not overwritten)\n", path)
	}
	fmt.Fprintln(cmd.Writer, "Ignore .tell.yaml in Git. Next: tell config check")
	return nil
}

func emit(cmd *urfave.Command, result map[string]any) error {
	result["schema_version"] = 1
	return json.NewEncoder(cmd.Writer).Encode(result)
}

// Match Noon's output-mode handling on parse errors without treating a string
// flag's value (for example --apprise-tag --json) as an output flag.
func jsonRequested(root *urfave.Command, args []string) bool {
	valueFlags := map[string]bool{}
	_ = root.Walk(func(cmd *urfave.Command) error {
		for _, flag := range cmd.Flags {
			if _, boolean := flag.(*urfave.BoolFlag); !boolean {
				for _, name := range flag.Names() {
					valueFlags[name] = true
				}
			}
		}
		return nil
	})
	requested := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if valueFlags[name] {
			if !hasValue {
				i++
			}
		} else if name == "json" {
			requested = true
			if hasValue {
				requested, _ = strconv.ParseBool(value)
			}
		}
	}
	return requested
}
