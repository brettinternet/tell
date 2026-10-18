// Package config resolves settings without executing providers or creating state.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BuildOpts contains metadata injected at link time.
type BuildOpts struct {
	Version, Commit, BuildTime string
}

// Value is an invocation override with its origin (flag or environment variable).
type Value struct {
	Text, Source string
}

// Input supplies invocation context. Nil selectors are absent, not empty.
// UserConfigDir is the base directory containing the tell directory. Paths are never shell-expanded.
type Input struct {
	CWD, UserConfigDir string
	Project, File      *string
	Overrides          map[string]Value
}

type Notify struct {
	Via string `json:"via"`
	// Command is trusted personal argv, never exposed in inspection output.
	Command []string `json:"-"`
}

type Secrets struct {
	Policy string `json:"policy"`
}

type Apprise struct {
	Config string `json:"config,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Format string `json:"format,omitempty"`
}

type Herdr struct {
	Session string `json:"session,omitempty"`
}

type Send struct {
	Via  string `json:"via,omitempty"`
	To   string `json:"to,omitempty"`
	From string `json:"from"`
}

type Listen struct {
	Slack  *Slack  `json:"slack,omitempty"`
	Routes []Route `json:"-"`
}
type Slack struct {
	AppTokenEnv string `json:"app_token_env"`
}

type Route struct {
	Name   string               `json:"-"`
	Source string               `json:"-"`
	Type   string               `json:"-"`
	Match  map[string]Predicate `json:"-"`
	Run    []string             `json:"-"`
}

type Predicate struct {
	Value  any
	Prefix *string
}

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var routeName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
var matchPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(?:\.[A-Za-z_][A-Za-z0-9_-]*)*$`)
var jsonNumber = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

func validateRouteSyntax(routes []Route) error {
	names := make(map[string]bool, len(routes))
	for _, route := range routes {
		if !routeName.MatchString(route.Name) {
			return errors.New("listen.routes names must match [a-z0-9][a-z0-9_-]*")
		}
		if names[route.Name] {
			return errors.New("listen.routes names must be unique")
		}
		names[route.Name] = true
		if route.Source != "slack" || route.Type == "" || len(route.Type) > 128 || strings.ContainsFunc(route.Type, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return errors.New("listen.routes source must be slack and type must be a nonempty safe value")
		}
		if len(route.Run) == 0 || strings.TrimSpace(route.Run[0]) == "" {
			return errors.New("listen.routes require one nonempty run argv action")
		}
		for _, arg := range route.Run {
			if strings.ContainsRune(arg, 0) {
				return errors.New("listen.routes run arguments must not contain NUL")
			}
		}
		for path, predicate := range route.Match {
			if !matchPath.MatchString(path) {
				return errors.New("listen.routes match paths must be dot-separated object paths")
			}
			if predicate.Prefix != nil {
				if predicate.Value != nil || strings.ContainsFunc(*predicate.Prefix, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
					return errors.New("listen.routes prefix predicates require a safe string")
				}
				continue
			}
			switch value := predicate.Value.(type) {
			case string, bool:
			case json.Number:
				if !jsonNumber.MatchString(string(value)) {
					return errors.New("listen.routes numeric predicates must be finite JSON numbers")
				}
			default:
				return errors.New("listen.routes predicates must be strings, finite numbers, booleans, or prefixes")
			}
		}
	}
	return nil
}

func validateRoutes(listen Listen) error {
	if err := validateRouteSyntax(listen.Routes); err != nil {
		return err
	}
	for _, route := range listen.Routes {
		switch route.Source {
		case "slack":
			if listen.Slack == nil {
				return errors.New("listen.routes reference slack, but listen.slack is not configured")
			}
		default:
			return errors.New("listen.routes reference an unconfigured source")
		}
	}
	return nil
}

type MCP struct {
	AgentTargets bool `json:"agent_targets"`
}

// Config is the effective snapshot. Provider files are referenced, never parsed.
// Origins tracks the winning source of each setting, including built-in defaults.
type Config struct {
	Project string            `json:"project"`
	Timeout string            `json:"timeout"`
	Notify  Notify            `json:"notify"`
	Secrets Secrets           `json:"secrets"`
	Apprise Apprise           `json:"apprise"`
	Herdr   Herdr             `json:"herdr"`
	Send    Send              `json:"send"`
	MCP     MCP               `json:"mcp"`
	Listen  Listen            `json:"listen"`
	Sources []string          `json:"sources"`
	Origins map[string]string `json:"origins"`
}

// Load merges user, shared-project, repository-personal, then invocation
// settings. File replaces automatic file sources, not invocation overrides.
func Load(input Input) (Config, error) {
	project, err := ProjectDir(input)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Project: project, Timeout: "10s", Notify: Notify{Via: "stderr"}, Secrets: Secrets{Policy: "redact"}, Send: Send{From: "tell-automation"},
		Sources: []string{}, Origins: map[string]string{},
	}
	for key := range cfg.Values() {
		cfg.Origins[key] = "default"
	}
	if input.File != nil {
		if strings.TrimSpace(*input.File) == "" {
			return Config{}, errors.New("config path must not be empty")
		}
		if err := cfg.applyFile(resolvePath(input.CWD, *input.File), false, false); err != nil {
			return Config{}, err
		}
	} else {
		if input.UserConfigDir != "" {
			if !filepath.IsAbs(input.UserConfigDir) {
				return Config{}, errors.New("user config directory must be absolute")
			}
			if err := cfg.applyFile(filepath.Join(input.UserConfigDir, "tell", "config.yaml"), true, false); err != nil {
				return Config{}, err
			}
		}
		if err := cfg.applyFile(filepath.Join(project, "tell.yaml"), true, true); err != nil {
			return Config{}, err
		}
		gitDir, err := gitCommonDir(project)
		if err != nil {
			return Config{}, err
		}
		if gitDir != "" {
			if err := cfg.applyFile(filepath.Join(gitDir, "tell.yaml"), true, false); err != nil {
				return Config{}, err
			}
		}
	}
	for key, value := range input.Overrides {
		if key == "secrets.policy" || key == "notify.command" || key == "listen" || strings.HasPrefix(key, "listen.") {
			return Config{}, fmt.Errorf("%s is settable only in personal config files", key)
		}
		if err := cfg.apply(key, value.Text, input.CWD, value.Source); err != nil {
			return Config{}, fmt.Errorf("%s: %w", strings.TrimPrefix(strings.TrimPrefix(value.Source, "flag:"), "env:"), err)
		}
	}
	if cfg.Listen.Slack != nil && cfg.Listen.Slack.AppTokenEnv == "" {
		return Config{}, errors.New("listen.slack.app_token_env is required")
	}
	if err := validateRoutes(cfg.Listen); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Values returns non-secret settings for config inspection, never raw command argv.
func (cfg Config) Values() map[string]string {
	command := ""
	if len(cfg.Notify.Command) > 0 {
		command = "[configured]"
	}
	return map[string]string{
		"timeout": cfg.Timeout, "notify.via": cfg.Notify.Via, "notify.command": command,
		"apprise.config": cfg.Apprise.Config, "apprise.tag": cfg.Apprise.Tag,
		"apprise.format": cfg.Apprise.Format, "herdr.session": cfg.Herdr.Session,
		"send.via": cfg.Send.Via, "send.to": cfg.Send.To, "send.from": cfg.Send.From, "secrets.policy": cfg.Secrets.Policy,
		"mcp.agent_targets":          fmt.Sprint(cfg.MCP.AgentTargets),
		"listen.slack.app_token_env": cfg.SlackTokenEnv(),
		"listen.routes":              strconv.Itoa(len(cfg.Listen.Routes)),
	}
}

func (cfg *Config) apply(key, value, base, source string) error {
	if strings.TrimSpace(value) == "" && key != "send.to" {
		return fmt.Errorf("%s must not be empty", key)
	}
	// Terminal control bytes and NUL have no role in these settings.
	if strings.ContainsAny(value, "\x00\x1b\r\n") {
		return fmt.Errorf("%s contains control characters", key)
	}
	switch key {
	case "listen":
	// Presence marker ensures even an empty shared block is rejected.
	case "listen.slack":
		if cfg.Listen.Slack == nil {
			cfg.Listen.Slack = &Slack{}
		}
	case "listen.slack.app_token_env":
		if !environmentName.MatchString(value) {
			return errors.New("listen.slack.app_token_env must be an environment variable name")
		}
		if cfg.Listen.Slack == nil {
			cfg.Listen.Slack = &Slack{}
		}
		cfg.Listen.Slack.AppTokenEnv = value
	case "secrets.policy":
		if value != "redact" && value != "reject" {
			return errors.New("secrets.policy must be redact or reject")
		}
		cfg.Secrets.Policy = value
	case "timeout":
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return errors.New("timeout must be a positive Go duration, such as 10s")
		}
		cfg.Timeout = value
	case "notify.via":
		switch value {
		case "stderr", "desktop", "apprise", "herdr", "command":
			cfg.Notify.Via = value
		default:
			return errors.New("notify.via must be stderr, desktop, apprise, herdr, or command")
		}
	case "apprise.config":
		if strings.Contains(value, "://") {
			return errors.New("apprise.config must be a local path, not a URL")
		}
		cfg.Apprise.Config = resolvePath(base, value)
	case "apprise.tag":
		cfg.Apprise.Tag = value
	case "apprise.format":
		switch value {
		case "text", "markdown", "html":
			cfg.Apprise.Format = value
		default:
			return errors.New("apprise.format must be text, markdown, or html")
		}
	case "herdr.session":
		cfg.Herdr.Session = value
	case "send.via":
		if value != "hcom" && value != "herdr" {
			return errors.New("send.via must be hcom or herdr")
		}
		cfg.Send.Via = value
	case "send.to":
		cfg.Send.To = value
	case "send.from":
		if !ValidIdentity(value) {
			return errors.New("send.from must be a simple identifier (letters, digits, underscores, hyphens; at most 64 bytes)")
		}
		cfg.Send.From = value
	case "mcp.agent_targets":
		targets, err := strconv.ParseBool(value)
		if err != nil {
			return errors.New("mcp.agent_targets must be true or false")
		}
		cfg.MCP.AgentTargets = targets
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
	cfg.Origins[key] = source
	return nil
}

func (cfg *Config) applyFile(path string, optional, shared bool) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if optional && errors.Is(err, os.ErrNotExist) {
		// Dangling symlinks are broken configuration, not absent optional files.
		if _, linkErr := os.Lstat(path); errors.Is(linkErr, os.ErrNotExist) {
			return nil
		}
	}
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("config %s: must be a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	defer file.Close()
	const maxSize = 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if len(data) > maxSize {
		return fmt.Errorf("config %s: exceeds 1 MiB", path)
	}
	values, err := parse(data)
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	for _, value := range values {
		if shared {
			if strings.HasPrefix(value.key, "listen") {
				return fmt.Errorf("config %s:%d: listen is personal configuration; use user config, .git/tell.yaml, or --config", path, value.line)
			}
			switch value.key {
			case "secrets.policy", "notify.command":
				return fmt.Errorf("config %s:%d: %s is personal policy; set it in user config, .git/tell.yaml, or --config instead of shared tell.yaml", path, value.line, value.key)
			case "notify.via", "apprise.config", "herdr.session", "send.via", "send.to", "send.from", "mcp.agent_targets":
				return fmt.Errorf("config %s:%d: %s is personal routing; set it in user config, .git/tell.yaml, --config, environment, or flags instead of shared tell.yaml", path, value.line, value.key)
			}
		}
		if value.key == "notify.command" {
			cfg.Notify.Command = value.argv
			cfg.Origins[value.key] = path
			continue
		}
		if value.key == "listen.routes" {
			cfg.Listen.Routes = append([]Route{}, value.routes...)
			cfg.Origins[value.key] = path
			continue
		}
		if err := cfg.apply(value.key, value.text, filepath.Dir(path), path); err != nil {
			return fmt.Errorf("config %s:%d: %w", path, value.line, err)
		}
	}
	cfg.Sources = append(cfg.Sources, path)
	return nil
}

// ProjectDir follows Noon's nearest config/Git boundary convention, including
// worktree .git files. An explicit project selects exactly that directory.
func ProjectDir(input Input) (string, error) {
	start := input.CWD
	if input.Project != nil {
		if strings.TrimSpace(*input.Project) == "" {
			return "", errors.New("project must not be empty")
		}
		start = resolvePath(input.CWD, *input.Project)
	}
	start, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	start, err = filepath.EvalSymlinks(start)
	if err != nil {
		return "", fmt.Errorf("project: %w", err)
	}
	info, err := os.Stat(start)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project %s is not a directory", start)
	}
	if input.Project != nil {
		return start, nil
	}
	for dir := start; ; dir = filepath.Dir(dir) {
		for _, name := range []string{"tell.yaml", ".git"} {
			if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
				return dir, nil
			} else if !errors.Is(err, os.ErrNotExist) {
				return "", fmt.Errorf("discover project: %w", err)
			}
		}
		if filepath.Dir(dir) == dir {
			return start, nil
		}
	}
}

// gitCommonDir returns the project's common Git directory, shared by linked
// worktrees, or "" when the project has no .git. Git refuses to commit paths
// inside it, so repository content cannot place personal config there. Only
// .git and commondir are read; git never runs.
func gitCommonDir(project string) (string, error) {
	marker := filepath.Join(project, ".git")
	info, err := os.Stat(marker)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("git directory %s: %w", marker, err)
	}
	gitDir := marker
	if !info.IsDir() {
		line, err := readGitPointer(marker)
		target, ok := strings.CutPrefix(line, "gitdir: ")
		if err == nil && (!ok || target == "") {
			err = errors.New("want a gitdir: line")
		}
		if err != nil {
			return "", fmt.Errorf("git directory %s: %w", marker, err)
		}
		gitDir = resolvePath(project, target)
	}
	// Linked worktrees name the shared directory in commondir; others are it.
	common, err := readGitPointer(filepath.Join(gitDir, "commondir"))
	if errors.Is(err, os.ErrNotExist) {
		return gitDir, nil
	}
	if err == nil && common == "" {
		err = errors.New("empty")
	}
	if err != nil {
		return "", fmt.Errorf("git directory %s: commondir: %w", gitDir, err)
	}
	return resolvePath(gitDir, common), nil
}

func readGitPointer(path string) (string, error) {
	// Opening a FIFO or device could block or read without bound.
	if info, err := os.Stat(path); err != nil {
		return "", err
	} else if !info.Mode().IsRegular() {
		return "", errors.New("must be a regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	const maxSize = 4096
	data, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSize {
		return "", errors.New("exceeds 4 KiB")
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}
