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
	"text/tabwriter"
	"unicode"

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
			&urfave.StringFlag{Name: "project", Aliases: []string{"C"}, Usage: "Select exactly this project directory [$TELL_PROJECT]"},
			&urfave.StringFlag{Name: "config", Aliases: []string{"c"}, Usage: "Replace all automatic config files with FILE [$TELL_CONFIG]"},
			&urfave.BoolFlag{Name: "json", Aliases: []string{"j"}, Usage: "Print one structured JSON result"},
		},
		Action: showConfig,
		Commands: []*urfave.Command{
			{Name: "send", Usage: "Send to an existing agent; dispatch is not completion", ArgsUsage: "MESSAGE... | -", Flags: []urfave.Flag{
				&urfave.StringFlag{Name: "intent", Aliases: []string{"i"}, Value: "inform", Usage: "Message intent: inform or request"},
			}, Action: send},
			{Name: "doctor", Usage: "Check configured routes without sending or starting services", Action: doctor},
			{Name: "mcp", Usage: "Serve notify, send, and doctor as MCP tools over stdio", Action: mcpCommand(build.Version)},
			{Name: "notify", Usage: "Notify via the selected route; acceptance is not human acknowledgment", ArgsUsage: "MESSAGE... | -", Flags: []urfave.Flag{
				&urfave.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Notification title (default: project directory name)"},
				&urfave.StringFlag{Name: "type", Aliases: []string{"n"}, Value: "info", Usage: "Notification type: info, success, warning, failure"},
			}, Action: notify},
			{Name: "config", Usage: "Inspect effective settings and origins without sending anything", Action: showConfig, Commands: []*urfave.Command{
				{Name: "show", Usage: "Show effective settings and origins", Action: showConfig},
				{Name: "check", Usage: "Validate settings without invoking providers", Action: checkConfig},
			}},
			{Name: "init", Usage: "Create tell.yaml without overwriting; never sends messages", Flags: []urfave.Flag{
				&urfave.BoolFlag{Name: "user", Aliases: []string{"u"}, Usage: "Create personal user config instead"},
			}, Action: initialize},
		},
	}
	_ = root.Walk(func(cmd *urfave.Command) error {
		for _, spec := range settings {
			include := false
			switch cmd.Name {
			case "tell":
				include = spec.key == "timeout"
			case "notify":
				include = strings.HasPrefix(spec.key, "notify.") || strings.HasPrefix(spec.key, "apprise.") || spec.key == "herdr.session"
			case "send":
				include = strings.HasPrefix(spec.key, "send.") || spec.key == "herdr.session"
			case "doctor", "config", "mcp":
				include = spec.key != "timeout"
			}
			if include {
				// Resolve env in input() to retain exact flag/env provenance.
				// Config subcommands inherit these flags from config.
				cmd.Flags = append(cmd.Flags, &urfave.StringFlag{Name: spec.flag, Usage: spec.usage + " [$" + spec.env + "]"})
			}
		}
		if cmd.Name == "doctor" || cmd.Name == "config" || cmd.Name == "mcp" {
			cmd.Flags = append(cmd.Flags, &urfave.BoolFlag{Name: "mcp-agent-targets", Usage: "Allow MCP agents to override route and recipient selectors [$TELL_MCP_AGENT_TARGETS]"})
		}
		cmd.HideHelpCommand = len(cmd.Commands) == 0
		cmd.OnUsageError = func(_ context.Context, _ *urfave.Command, err error, _ bool) error {
			message := err.Error()
			// Parser values can be message text or credentials. Keep the flag
			// and cause, but do not repeat arbitrary rejected boolean values.
			if strings.HasPrefix(message, "invalid value ") {
				if index := strings.LastIndex(message, " for flag "); index >= 0 {
					message = "invalid value [omitted]" + message[index:]
				}
			}
			return &failure{Code: "usage", Message: delivery.RedactDiagnostic(message) + "; run " + cmd.FullName() + " --help", usage: true}
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
	args = preserveMessageDash(root, args)
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
				result = delivery.NewResult("", "usage", "invalid_invocation", err.Error())
			}
			renderSend(result, jsonRequested(root, args), out, errOut)
			return result.ExitCode()
		}
		if !errors.As(err, &result) && notifyRequested(root, args) {
			result = delivery.NewResult("", "usage", "invalid_invocation", err.Error())
		}
		if result != nil {
			if jsonRequested(root, args) {
				_ = json.NewEncoder(out).Encode(result)
			} else {
				fmt.Fprintf(errOut, "tell: %s\n", humanReason(result.Route, result.Failure.Reason, result.Error()))
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
	values := config.Input{CWD: cwd, Project: selector(cmd, "project", "TELL_PROJECT"), File: selector(cmd, "config", "TELL_CONFIG"), Overrides: map[string]config.Value{}}
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

// selector resolves env here, like settings, so IsSet reports only flags.
func selector(cmd *urfave.Command, name, env string) *string {
	if cmd.IsSet(name) {
		value := cmd.String(name)
		return &value
	}
	if value, exists := os.LookupEnv(env); exists {
		return &value
	}
	return nil
}

// A mistyped option must not cause the message itself to enter diagnostics.
func usageValue(cmd *urfave.Command, value string) string {
	for _, body := range cmd.Args().Slice() {
		if value == body {
			return "[message omitted]"
		}
	}
	return delivery.RedactDiagnostic(value)
}

func noArgs(cmd *urfave.Command) error {
	if cmd.NArg() != 0 {
		message := fmt.Sprintf("unexpected argument %q; run tell --help", cmd.Args().First())
		if cmd.Name == "tell" {
			message = fmt.Sprintf("unknown command %q", delivery.RedactDiagnostic(cmd.Args().First()))
			if suggestion := urfave.SuggestCommand(cmd.Commands, cmd.Args().First()); suggestion != "" {
				message += fmt.Sprintf("; did you mean %q?", suggestion)
			}
			message += "; run tell --help"
		}
		return &failure{Code: "usage", Message: message, usage: true}
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
	return config.Config{}, &failure{Code: "config", Message: delivery.RedactDiagnostic(err.Error())}
}

func showConfig(_ context.Context, cmd *urfave.Command) error {
	cfg, err := load(cmd)
	if err != nil {
		return err
	}
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"config": cfg})
	}
	fmt.Fprintf(cmd.Writer, "Project: %s\n", displayValue(cfg.Project))
	if cmd.Name == "tell" {
		fmt.Fprintln(cmd.Writer, "Config files:")
	} else {
		fmt.Fprintln(cmd.Writer, "Files (low → high precedence):")
	}
	if len(cfg.Sources) == 0 {
		fmt.Fprintln(cmd.Writer, "  none")
	}
	for _, source := range cfg.Sources {
		fmt.Fprintf(cmd.Writer, "  %s\n", displayValue(source))
	}
	if cmd.Name == "tell" {
		fmt.Fprintf(cmd.Writer, "Notify: %s (%s)\n", cfg.Notify.Via, displayOrigin(cfg.Origins["notify.via"]))
		if cfg.Send.Via == "" || cfg.Send.To == "" {
			fmt.Fprintln(cmd.Writer, "Send: not configured")
		} else {
			fmt.Fprintf(cmd.Writer, "Send: %s → %s\n", cfg.Send.Via, displayValue(cfg.Send.To))
		}
		fmt.Fprintln(cmd.Writer, "\nNext: tell notify \"hello\" · tell doctor · tell init · tell --help")
		return nil
	}
	values := cfg.Values()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	table := tabwriter.NewWriter(cmd.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "\nKEY\tVALUE\tORIGIN")
	for _, key := range keys {
		fmt.Fprintf(table, "%s\t%s\t%s\n", key, displayValue(values[key]), displayOrigin(cfg.Origins[key]))
	}
	return table.Flush()
}

// Quote whitespace that would otherwise be invisible or disrupt the table.
func displayValue(value string) string {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) || strings.Contains(value, "  ") {
		return strconv.Quote(value)
	}
	return value
}

func displayOrigin(origin string) string {
	for _, prefix := range []string{"env:", "flag:"} {
		if value, ok := strings.CutPrefix(origin, prefix); ok {
			return strings.TrimSuffix(prefix, ":") + " " + displayValue(value)
		}
	}
	return displayValue(origin)
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
	// Ambient TELL_* environment is routine in shells and services; only typed
	// flags suggest the user expects init to write those values.
	overrides := []string{"config", "mcp-agent-targets"}
	for _, spec := range settings {
		overrides = append(overrides, spec.flag)
	}
	if slices.ContainsFunc(overrides, cmd.IsSet) {
		return &failure{Code: "usage", Message: "init does not apply config or setting overrides; edit the generated file instead", usage: true}
	}
	if cmd.Bool("user") && cmd.IsSet("project") {
		return &failure{Code: "usage", Message: "--user cannot be combined with --project", usage: true}
	}
	var path string
	var created bool
	if cmd.Bool("user") {
		values.UserConfigDir, err = userConfigDir()
		if err == nil {
			path, created, err = config.InitUser(values)
		}
	} else {
		path, created, err = config.Init(values)
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
	fmt.Fprintln(cmd.Writer, "Next: tell config check")
	return nil
}

func emit(cmd *urfave.Command, result map[string]any) error {
	result["schema_version"] = 1
	return json.NewEncoder(cmd.Writer).Encode(result)
}

// urfave v3.13 drops arguments after a bare dash. Protect a message dash
// with the end-of-options marker, without changing dashes used as flag values.
func preserveMessageDash(root *urfave.Command, args []string) []string {
	if !notifyRequested(root, args) && !commandRequested(root, args, "send") {
		return args
	}
	valueFlags := commandValueFlags(root)
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return args
		}
		if strings.TrimSpace(arg) == "-" {
			return slices.Insert(slices.Clone(args), i, "--")
		}
		if strings.HasPrefix(arg, "-") {
			name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
			if valueFlags[name] && !hasValue {
				i++
			}
		}
	}
	return args
}

func commandValueFlags(root *urfave.Command) map[string]bool {
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
	return valueFlags
}

// Match Noon's output-mode handling on parse errors without treating a string
// flag's value (for example --apprise-tag --json) as an output flag.
func jsonRequested(root *urfave.Command, args []string) bool {
	valueFlags := commandValueFlags(root)
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
		} else if name == "json" || name == "j" {
			requested = true
			if hasValue {
				requested, _ = strconv.ParseBool(value)
			}
		}
	}
	return requested
}
