// Package config resolves settings without executing providers or creating state.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Sources []string          `json:"sources"`
	Origins map[string]string `json:"origins"`
}

// Load merges user, shared-project, private-project, then invocation
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
		for _, name := range []string{"tell.yaml", ".tell.yaml"} {
			if err := cfg.applyFile(filepath.Join(project, name), true, name == "tell.yaml"); err != nil {
				return Config{}, err
			}
		}
	}
	for key, value := range input.Overrides {
		if key == "secrets.policy" || key == "notify.command" {
			return Config{}, fmt.Errorf("%s is settable only in personal config files", key)
		}
		if err := cfg.apply(key, value.Text, input.CWD, value.Source); err != nil {
			return Config{}, fmt.Errorf("%s: %w", value.Source, err)
		}
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
			switch value.key {
			case "secrets.policy", "notify.command":
				return fmt.Errorf("config %s:%d: %s is personal policy; set it in user config, .tell.yaml, or --config instead of shared tell.yaml", path, value.line, value.key)
			case "notify.via", "apprise.config", "herdr.session", "send.via", "send.to", "send.from":
				return fmt.Errorf("config %s:%d: %s is personal routing; set it in user config, .tell.yaml, --config, environment, or flags instead of shared tell.yaml", path, value.line, value.key)
			}
		}
		if value.key == "notify.command" {
			cfg.Notify.Command = value.argv
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
		for _, name := range []string{"tell.yaml", ".tell.yaml", ".git"} {
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

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}
