package config_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/config"
)

func TestGitHubPersonalConfigurationAndCredentialIsolation(t *testing.T) {
	input := fixture(t)
	file := filepath.Join(input.CWD, "github.yaml")
	input.File = &file
	writeFile(t, file, `listen:
  github:
    secret_env: TELL_GITHUB_SECRET
    repository_ids: [12345, 67890]
  routes:
    - name: opened-pr
      source: github
      type: pull_request
      match: {action: opened}
      run: [/bin/true]
`)
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GitHubSecretEnv() != "TELL_GITHUB_SECRET" || cfg.HTTPAddr() != "127.0.0.1:8787" || !reflect.DeepEqual(cfg.Listen.GitHub.RepositoryIDs, []int64{12345, 67890}) {
		t.Fatalf("GitHub config/defaults were not loaded: %+v", cfg.Listen)
	}
	if cfg.Values()["listen.github.repository_ids"] != "12345,67890" {
		t.Fatalf("repository allowlist was not inspectable: %+v", cfg.Values())
	}
	t.Setenv("TELL_GITHUB_SECRET", "sensitive-secret-value")
	child := cfg.ChildEnv([]string{"TELL_GITHUB_SECRET=sensitive-secret-value", "KEEP=visible"})
	if !reflect.DeepEqual(child, []string{"KEEP=visible"}) {
		t.Fatalf("inbound credential reached child: %q", child)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "sensitive-secret-value") || strings.Contains(string(encoded), "raw-body") {
		t.Fatalf("secret leaked from configuration snapshot: %s", encoded)
	}

	for _, shared := range []string{
		"listen: {}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: [1]}}\n",
		"listen: {http: {addr: '127.0.0.1:8788'}}\n",
	} {
		input := fixture(t)
		writeFile(t, filepath.Join(input.CWD, "tell.yaml"), shared)
		if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "personal") {
			t.Fatalf("shared inbound config accepted: %v", err)
		}
	}
}

func TestGitHubConfigurationRejectsUnsafeBindingsAndAllowLists(t *testing.T) {
	input := fixture(t)
	file := filepath.Join(input.CWD, "invalid-github.yaml")
	input.File = &file
	invalid := []string{
		"listen: {github: {secret_env: TOKEN}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: []}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: [0]}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: [-1]}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: [1, 1]}}\n",
		"listen: {github: {secret_env: 'literal-secret', repository_ids: [1]}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: ['1']}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: [1.5]}}\n",
		"listen: {github: {secret_env: TOKEN, repository_ids: [9223372036854775808]}}\n",
	}
	for _, data := range invalid {
		writeFile(t, file, data)
		if _, err := config.Load(input); err == nil || strings.Contains(err.Error(), "literal-secret") {
			t.Errorf("invalid GitHub config accepted or value leaked (%q): %v", data, err)
		}
	}
	for _, addr := range []string{"0.0.0.0:8787", "192.168.1.5:8787", "localhost:8787", "127.0.0.1:0", "127.0.0.1:65536", "[::]:8787", "[fe80::1]:8787", "127.0.0.1", "127.0.0.1:08787", "127.0.0.1:+8787"} {
		writeFile(t, file, "listen: {http: {addr: "+addr+"}}\n")
		if _, err := config.Load(input); err == nil {
			t.Errorf("unsafe HTTP bind accepted: %q", addr)
		}
	}
	writeFile(t, file, "listen: {http: {addr: '[::1]:8787'}}\n")
	if cfg, err := config.Load(input); err != nil || cfg.HTTPAddr() != "[::1]:8787" {
		t.Fatalf("literal IPv6 loopback rejected: %+v %v", cfg.Listen.HTTP, err)
	}

	input = fixture(t)
	writeFile(t, filepath.Join(input.UserConfigDir, "tell", "config.yaml"), "listen: {http: {addr: '0.0.0.0:8787'}}\n")
	local := filepath.Join(input.CWD, "..", "worktree-metadata", "tell.yaml")
	writeFile(t, local, "listen: {http: {addr: '127.0.0.1:8788'}}\n")
	if _, err := config.Load(input); err == nil {
		t.Fatal("a higher layer overrode an invalid lower-layer bind")
	}
}

func TestGitHubRoutesRequireConfiguredSource(t *testing.T) {
	input := fixture(t)
	file := filepath.Join(input.CWD, "orphan-route.yaml")
	input.File = &file
	writeFile(t, file, `listen:
  routes:
    - name: orphan
      source: github
      type: pull_request
      run: [/bin/true]
`)
	if _, err := config.Load(input); err == nil {
		t.Fatal("accepted a GitHub route without a configured source")
	}
}
