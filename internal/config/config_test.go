package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/config"
)

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) config.Input {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(base, "repo")
	writeFile(t, filepath.Join(project, ".git"), "gitdir: ../worktree-metadata\n")
	system := filepath.Join(base, "system.yaml")
	writeFile(t, system, "version: 1\n")
	return config.Input{CWD: project, SystemFile: &system, UserConfigDir: filepath.Join(base, "user")}
}

// A higher-priority destination must win without rebasing inherited provider
// paths against the checkout or discarding other fields from the same section.
func TestLoadLayersAndPaths(t *testing.T) {
	input := fixture(t)
	writeFile(t, *input.SystemFile, "timeout: 3s\nnotify: {via: desktop}\n")
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, user, "apprise:\n  config: ../apprise/notifications.yaml\n  tag: personal\n  format: markdown\n")
	shared := filepath.Join(input.CWD, "tell.yaml")
	writeFile(t, shared, "notify: {via: apprise}\napprise: {tag: team}\n")
	private := filepath.Join(input.CWD, ".tell.yaml")
	writeFile(t, private, "apprise: {tag: local}\n")
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != "3s" || cfg.Notify.Via != "apprise" || cfg.Apprise.Tag != "local" || cfg.Apprise.Format != "markdown" {
		t.Fatalf("incorrect precedence: %+v", cfg)
	}
	wantPath := filepath.Join(input.UserConfigDir, "apprise", "notifications.yaml")
	if cfg.Apprise.Config != wantPath || cfg.Origins["apprise.config"] != user || cfg.Origins["apprise.tag"] != private {
		t.Fatalf("lost path provenance: %+v", cfg)
	}
	if strings.Join(cfg.Sources, "|") != strings.Join([]string{*input.SystemFile, user, shared, private}, "|") {
		t.Fatalf("wrong sources: %v", cfg.Sources)
	}
	input.Overrides = map[string]config.Value{"apprise.config": {Text: "provider.yaml", Source: "flag:--apprise-config"}}
	cfg, err = config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Apprise.Config != filepath.Join(input.CWD, "provider.yaml") || cfg.Origins["apprise.config"] != "flag:--apprise-config" {
		t.Fatalf("invocation path resolved incorrectly: %+v", cfg)
	}
}

// --config must not load broken automatic sources or accidentally inherit a
// personal destination in CI. Overrides still apply to the selected file.
func TestExplicitConfigIsolatesAutomaticFiles(t *testing.T) {
	input := fixture(t)
	writeFile(t, *input.SystemFile, "invalid: true\n")
	writeFile(t, filepath.Join(input.CWD, "tell.yaml"), "apprise: {tag: unexpected}\n")
	explicit := filepath.Join(input.CWD, "ci.yaml")
	writeFile(t, explicit, "timeout: 7s\n")
	input.File = &explicit
	input.Overrides = map[string]config.Value{"timeout": {Text: "9s", Source: "env:TELL_TIMEOUT"}}
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Apprise.Tag != "" || cfg.Timeout != "9s" || len(cfg.Sources) != 1 || cfg.Sources[0] != explicit {
		t.Fatalf("automatic settings leaked: %+v", cfg)
	}
}

// Typos, duplicate fields and YAML indirection can silently redirect messages.
// Validate each source even if a later layer supplies a valid replacement.
func TestRejectAmbiguousOrInvalidConfiguration(t *testing.T) {
	cases := []struct{ name, data string }{
		{"unknown nested key", "apprise: {tags: ops}\n"},
		{"duplicate nested key", "apprise: {tag: first, tag: second}\n"},
		{"dotted alias for nested key", "notify.via: apprise\nnotify: {via: herdr}\n"},
		{"alias", "notify: &route {via: apprise}\napprise: *route\n"},
		{"merge", "notify: {<<: {via: herdr}}\n"},
		{"null", "apprise: {tag: null}\n"},
		{"wrong type", "apprise: {tag: [ops]}\n"},
		{"multiple documents", "timeout: 1s\n---\ntimeout: 2s\n"},
		{"unsupported version", "version: 2\n"},
		{"unbounded timeout", "timeout: 0s\n"},
		{"credential URL", "apprise: {config: 'https://user:secret-sentinel@example.com/config'}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := fixture(t)
			writeFile(t, *input.SystemFile, tc.data)
			writeFile(t, filepath.Join(input.CWD, ".tell.yaml"), "timeout: 2s\nnotify: {via: stdout}\napprise: {tag: valid}\n")
			_, err := config.Load(input)
			if err == nil || !strings.Contains(err.Error(), *input.SystemFile) {
				t.Fatalf("expected source diagnostic, got %v", err)
			}
			if strings.Contains(err.Error(), "secret-sentinel") {
				t.Fatalf("diagnostic leaked a raw value: %v", err)
			}
		})
	}
}

// Worktrees use .git files: discovery must not walk past them and load a parent
// project's notification policy. An explicit -C also must not walk upward.
func TestProjectBoundaries(t *testing.T) {
	input := fixture(t)
	writeFile(t, filepath.Join(filepath.Dir(input.CWD), "tell.yaml"), "notify: {via: herdr}\n")
	repo := input.CWD
	input.CWD = filepath.Join(repo, "nested")
	if err := os.Mkdir(input.CWD, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project != repo || cfg.Notify.Via != "stdout" {
		t.Fatalf("escaped Git boundary: %+v", cfg)
	}
	input.Project = &input.CWD
	cfg, err = config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project != input.CWD {
		t.Fatalf("explicit project walked upward: %s", cfg.Project)
	}
}

// A dangling private-config symlink is a broken override, not permission to
// silently fall back to a shared destination. Non-files must not be consumed.
func TestBrokenConfigPathsFail(t *testing.T) {
	input := fixture(t)
	private := filepath.Join(input.CWD, ".tell.yaml")
	if err := os.Symlink(filepath.Join(input.CWD, "missing.yaml"), private); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(input); err == nil {
		t.Fatal("ignored dangling private config")
	}
	directory := input.CWD
	input.File = &directory
	if _, err := config.Load(input); err == nil {
		t.Fatal("accepted a directory as config")
	}
}

// Init must not destroy local routing/credentials or follow a planted symlink.
func TestInitPreservesExistingFiles(t *testing.T) {
	input := fixture(t)
	path, created, err := config.Init(input, true)
	if err != nil || !created {
		t.Fatalf("init: %t %v", created, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private mode: %o", info.Mode().Perm())
	}
	writeFile(t, path, "private-sentinel\n")
	_, created, err = config.Init(input, true)
	if err != nil || created {
		t.Fatalf("repeat init: %t %v", created, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "private-sentinel\n" {
		t.Fatal("init changed the existing file")
	}
	shared := filepath.Join(input.CWD, "tell.yaml")
	if err := os.Symlink(path, shared); err != nil {
		t.Fatal(err)
	}
	if _, _, err := config.Init(input, false); err == nil {
		t.Fatal("init accepted a symlink")
	}
}
