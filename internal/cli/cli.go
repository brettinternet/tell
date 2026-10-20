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

type secondSignalKey struct{}

// WithSecondSignal gives live listeners a second process signal to cancel
// running actions after the first signal begins graceful shutdown.
func WithSecondSignal(ctx context.Context, signals <-chan os.Signal) context.Context {
	return context.WithValue(ctx, secondSignalKey{}, signals)
}

func shutdownSignal(ctx context.Context) <-chan os.Signal {
	signals, _ := ctx.Value(secondSignalKey{}).(<-chan os.Signal)
	return signals
}

func init() {
	urfave.HelpPrinter = printHelp
	urfave.VersionPrinter = func(cmd *urfave.Command) {
		fmt.Fprintf(cmd.Writer, "%s %s\n", cmd.Name, cmd.Version)
	}
}

type settingSpec struct{ key, flag, env, usage string }

var settings = []settingSpec{
	{"timeout", "timeout", "TELL_TIMEOUT", "Wait up to a positive Go `duration`, such as 5s or 1m (default: 10s)"},
	{"notify.via", "via", "TELL_NOTIFY_VIA", "Notification `route`: stderr, desktop, apprise, command, herdr (default: stderr)"},
	{"apprise.config", "apprise-config", "TELL_APPRISE_CONFIG", "Local Apprise config `file`; Tell does not read it (default: Apprise discovery)"},
	{"apprise.tag", "apprise-tag", "TELL_APPRISE_TAG", "Apprise tag `expression` (default: no tag filter)"},
	{"apprise.format", "apprise-format", "TELL_APPRISE_FORMAT", "Apprise body `format`: text, markdown, html (default: provider setting)"},
	{"herdr.session", "herdr-session", "TELL_HERDR_SESSION", "Herdr `session` name or ID (default: unset; required for herdr)"},
	{"send.via", "send-via", "TELL_SEND_VIA", "Agent `route`: hcom or herdr (default: unset)"},
	{"send.to", "to", "TELL_SEND_TO", "One exact agent `recipient`, without @ (default: unset)"},
	{"send.from", "from", "TELL_SEND_FROM", "Sender `name` (default: tell-automation)"},
}

// NewRootCommand constructs a fresh tree without reading files or executing tools.
func NewRootCommand(build config.BuildOpts, out, errOut io.Writer) *urfave.Command {
	root := &urfave.Command{
		Name: "tell", Usage: "Send a notification to a person or a message to an agent",
		Description: "Send one message, then exit. With no command, inspect settings without sending.\nAccepted means handed to the provider, not read.\nDefaults shown in help apply unless overridden by configuration or environment.",
		Version:     fmt.Sprintf("%s (%.7s, built %s)", build.Version, build.Commit, build.BuildTime),
		Writer:      out, ErrWriter: errOut, EnableShellCompletion: true,
		ExitErrHandler: func(context.Context, *urfave.Command, error) {},
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "project", Aliases: []string{"C"}, Usage: "Use this project `directory` (default: discover from current directory) [$TELL_PROJECT]"},
			&urfave.StringFlag{Name: "config", Aliases: []string{"c"}, Usage: "Use only this config `file` (default: user config, tell.yaml, and .git/tell.yaml) [$TELL_CONFIG]"},
			&urfave.BoolFlag{Name: "json", Aliases: []string{"j"}, Usage: "Print one structured JSON result"},
			&urfave.BoolFlag{Name: "no-color", Usage: "Disable color and bold styling [$NO_COLOR]"},
		},
		Action: showConfig,
		Commands: []*urfave.Command{
			listenCommand(),
			{Name: "version", Usage: "Print version and build information", Action: func(_ context.Context, cmd *urfave.Command) error {
				urfave.ShowVersion(cmd.Root())
				return nil
			}},
			{Name: "notify", Usage: "Send a notification to a person", ArgsUsage: "MESSAGE... | -",
				Description: "Join message words with spaces, or use a lone - to read stdin.\n\nExamples:\n  tell notify Build finished\n  tell notify --via stderr --type success --title Tests All passed\n  printf 'Build finished' | tell notify -",
				Flags: []urfave.Flag{
					&urfave.StringFlag{Name: "title", Aliases: []string{"t"}, Usage: "Notification `title` (default: project directory name)"},
					&urfave.StringFlag{Name: "type", Aliases: []string{"n"}, Value: "info", DefaultText: "info", Usage: "Notification `type`: info, success, warning, failure"},
				}, Action: notify},
			{Name: "send", Usage: "Send a message to an existing agent", ArgsUsage: "MESSAGE... | -",
				Description: "Use a configured recipient, or select one with --to. The recipient must already exist.\nJoin message words with spaces, or use a lone - to read stdin.\n\nExamples (with a participating hcom agent named reviewer):\n  tell send --send-via hcom --to reviewer Tests passed\n  tell send --send-via hcom --to reviewer --intent request Review the changes",
				Flags: []urfave.Flag{
					&urfave.StringFlag{Name: "intent", Aliases: []string{"i"}, Value: "inform", DefaultText: "inform", Usage: "Message `intent`: inform or request"},
				}, Action: send},
			{Name: "doctor", Usage: "Check route readiness without sending or starting services",
				Description: "Only the selected routes decide the exit code.\n\nExamples:\n  tell doctor\n  tell doctor --via desktop\n  tell --json doctor --via stderr", Action: doctor},
			{Name: "config", Usage: "Show settings and where they came from",
				Description: "Inspect or validate configuration without sending or running providers.\n\nExamples:\n  tell config\n  tell config check\n  tell --json config show", Action: showConfig, Commands: []*urfave.Command{
					{Name: "show", Usage: "Show effective settings and origins", Action: showConfig},
					{Name: "check", Usage: "Validate settings without invoking providers", Action: checkConfig},
				}},
			{Name: "init", Usage: "Create a starter config without overwriting files",
				Description: "Create shared tell.yaml, personal user config, or repository-personal .git/tell.yaml. Never sends messages.\n\nExamples:\n  tell init\n  tell init --user\n  tell init --local", Flags: []urfave.Flag{
					&urfave.BoolFlag{Name: "user", Aliases: []string{"u"}, Usage: "Create personal user config instead"},
					&urfave.BoolFlag{Name: "local", Aliases: []string{"l"}, Usage: "Create personal .git/tell.yaml for this repository and its worktrees instead"},
				}, Action: initialize},
			{Name: "mcp", Usage: "Serve notify, send, and doctor as MCP tools over stdio", Action: mcpCommand(build.Version)},
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
	// Help can render before parsing reaches a later opt-out (e.g. after an
	// unknown flag). Make the invocation's plain-output intent available there.
	root.Metadata = map[string]any{"plain-output": jsonRequested(root, args) || boolRequested(root, args, "no-color")}
	if err := root.Run(ctx, args); err != nil {
		var listenerError *listenerFailure
		if errors.As(err, &listenerError) {
			return 1
		}
		plain := jsonRequested(root, args) || boolRequested(root, args, "no-color") || commandRequested(root, args, "mcp") || commandRequested(root, args, "completion")
		style := newOutputStyle(errOut, plain)
		var report *doctorReport
		if errors.As(err, &report) {
			return report.render(root, args, out, errOut)
		}
		var outputError *resultWriteError
		if errors.As(err, &outputError) {
			// Delivery was accepted. A broken result stream must not invite a resend
			// by reporting usage/config (both promise nothing was sent).
			fmt.Fprintf(errOut, "%s %s\n", style.warning("tell:"), outputError.Error())
			return 0
		}
		var result *delivery.Result
		if commandRequested(root, args, "send") {
			if !errors.As(err, &result) {
				result = delivery.NewResult("", "usage", "invalid_invocation", err.Error())
			}
			renderSend(result, jsonRequested(root, args), out, errOut, style)
			return result.ExitCode()
		}
		if !errors.As(err, &result) && notifyRequested(root, args) {
			result = delivery.NewResult("", "usage", "invalid_invocation", err.Error())
		}
		if result != nil {
			if jsonRequested(root, args) {
				_ = json.NewEncoder(out).Encode(result)
			} else {
				prefix := style.failure("tell:")
				if result.Outcome == "uncertain" {
					prefix = style.warning("tell:")
				}
				fmt.Fprintf(errOut, "%s %s\n", prefix, humanReason(result.Route, result.Failure.Reason, result.Error()))
				if commandRequested(root, args, "notify") && len(result.Attempts) > 1 {
					for _, attempt := range result.Attempts {
						fmt.Fprintf(errOut, "  %s: %s", attempt.Label, attempt.Outcome)
						if attempt.Reason != "" {
							fmt.Fprintf(errOut, " (%s)", attempt.Reason)
						}
						fmt.Fprintln(errOut)
					}
				}
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
			fmt.Fprintf(errOut, "%s %s\n", style.failure("tell:"), issue.Message)
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
	style := commandStyle(cmd, cmd.Writer)
	fmt.Fprintf(cmd.Writer, "%s %s\n", style.bold("Project:"), displayValue(cfg.Project))
	if cmd.Name == "tell" {
		fmt.Fprintln(cmd.Writer, style.bold("Config files:"))
	} else {
		fmt.Fprintln(cmd.Writer, style.bold("Files (low → high precedence):"))
	}
	if len(cfg.Sources) == 0 {
		fmt.Fprintln(cmd.Writer, "  none")
	}
	for _, source := range cfg.Sources {
		fmt.Fprintf(cmd.Writer, "  %s\n", displayValue(source))
	}
	if cmd.Name == "tell" {
		fmt.Fprintf(cmd.Writer, "%s %s (%s)\n", style.bold("Notify:"), notifySummary(cfg), displayOrigin(cfg.Origins["notify.via"]))
		if cfg.Send.Via == "" || cfg.Send.To == "" {
			fmt.Fprintf(cmd.Writer, "%s not configured\n", style.bold("Send:"))
		} else {
			fmt.Fprintf(cmd.Writer, "%s %s → %s\n", style.bold("Send:"), cfg.Send.Via, displayValue(cfg.Send.To))
		}
		if cfg.Listen.Slack != nil {
			fmt.Fprintf(cmd.Writer, "Listen: slack (inspect with tell listen)\n")
		}
		if cfg.Listen.GitHub != nil {
			fmt.Fprintf(cmd.Writer, "Listen: github (inspect with tell listen)\n")
		}
		fmt.Fprintf(cmd.Writer, "\n%s tell notify \"hello\" · tell doctor · tell init · tell --help\n", style.bold("Next:"))
		return nil
	}
	values := cfg.Values()
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var rendered strings.Builder
	table := tabwriter.NewWriter(&rendered, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "KEY\tVALUE\tORIGIN")
	for _, key := range keys {
		fmt.Fprintf(table, "%s\t%s\t%s\n", key, displayValue(values[key]), displayOrigin(cfg.Origins[key]))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	// Style after alignment so escape sequences cannot widen the header cells.
	header, rows, _ := strings.Cut(rendered.String(), "\n")
	_, err = fmt.Fprintf(cmd.Writer, "\n%s\n%s", style.bold(header), rows)
	return err
}

// Quote whitespace that would otherwise be invisible or disrupt the table.
func displayValue(value string) string {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsFunc(value, unicode.IsControl) || strings.Contains(value, "  ") {
		return strconv.Quote(value)
	}
	return value
}

func notifySummary(cfg config.Config) string {
	if len(cfg.Notify.Steps) == 0 {
		return cfg.Notify.Via
	}
	steps, err := config.EffectiveNotifySteps(cfg)
	if err != nil {
		return cfg.Notify.Via
	}
	labels := make([]string, len(steps))
	for i, step := range steps {
		labels[i] = config.NotifyStepLabel(step)
	}
	return strings.Join(labels, " → ")
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
	fmt.Fprintf(cmd.Writer, "Configuration %s · %q\n", commandStyle(cmd, cmd.Writer).success("valid"), cfg.Project)
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
	if cmd.Bool("user") && (cmd.Bool("local") || cmd.IsSet("project")) {
		return &failure{Code: "usage", Message: "--user cannot be combined with --local or --project", usage: true}
	}
	var path string
	var created bool
	switch {
	case cmd.Bool("user"):
		values.UserConfigDir, err = userConfigDir()
		if err == nil {
			path, created, err = config.InitUser(values)
		}
	case cmd.Bool("local"):
		path, created, err = config.InitLocal(values)
	default:
		path, created, err = config.Init(values)
	}
	if err != nil {
		return err
	}
	if cmd.Bool("json") {
		return emit(cmd, map[string]any{"path": path, "created": created})
	}
	style := commandStyle(cmd, cmd.Writer)
	if created {
		fmt.Fprintf(cmd.Writer, "%s %q\n", style.success("Created"), path)
	} else {
		fmt.Fprintf(cmd.Writer, "%s %q (not overwritten)\n", style.bold("Kept existing"), path)
	}
	if cmd.Bool("user") || cmd.Bool("local") {
		fmt.Fprintf(cmd.Writer, "%s set notify.via in this file, then run tell doctor.\n", style.bold("Next:"))
	} else {
		fmt.Fprintf(cmd.Writer, "%s commit tell.yaml. Run tell init --user to configure personal routing.\n", style.bold("Next:"))
	}
	return nil
}

func emit(cmd *urfave.Command, result map[string]any) error {
	result["schema_version"] = 1
	return json.NewEncoder(cmd.Writer).Encode(result)
}

// urfave v3.13 drops arguments after a bare dash. When a message word is a
// dash, move command flags ahead of an end-of-options marker so flags after
// the dash still parse as flags, as they do for any other message word.
func preserveMessageDash(root *urfave.Command, args []string) []string {
	if !notifyRequested(root, args) && !commandRequested(root, args, "send") {
		return args
	}
	valueFlags := commandValueFlags(root)
	// Skip root flags; commandRequested guarantees the command follows.
	start := 1
	for start < len(args) && strings.HasPrefix(args[start], "-") && args[start] != "--" {
		name, _, hasValue := strings.Cut(strings.TrimLeft(args[start], "-"), "=")
		if valueFlags[name] && !hasValue {
			start++
		}
		start++
	}
	if start >= len(args) || args[start] == "--" {
		return args
	}
	var flags, words []string
	dash := false
scan:
	for i := start + 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			words = append(words, args[i+1:]...)
			break scan
		case strings.TrimSpace(arg) == "-":
			dash = true
			words = append(words, arg)
		case strings.HasPrefix(arg, "-"):
			flags = append(flags, arg)
			name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
			if valueFlags[name] && !hasValue && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		default:
			words = append(words, arg)
		}
	}
	if !dash {
		return args
	}
	return slices.Concat(args[:start+1], flags, []string{"--"}, words)
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
	return boolRequested(root, args, "json", "j")
}

func boolRequested(root *urfave.Command, args []string, names ...string) bool {
	valueFlags := commandValueFlags(root)
	requested := false
	scope := root
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			// urfave may still dispatch a child after its parent's terminator.
			// A terminator within a leaf command protects message words instead.
			if scope != nil && i+1 < len(args) && scope.Command(args[i+1]) != nil {
				continue
			}
			break
		}
		if !strings.HasPrefix(arg, "-") {
			if scope != nil {
				scope = scope.Command(arg)
			}
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if valueFlags[name] {
			if !hasValue {
				i++
			}
		} else if slices.Contains(names, name) {
			if !hasValue {
				requested = true
			} else if parsed, err := strconv.ParseBool(value); err == nil {
				requested = parsed
			}
			// A rejected boolean must not undo an earlier valid output choice.
		}
	}
	return requested
}
