package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/cli"
	"github.com/brettinternet/tell/internal/config"
)

func environment(t *testing.T) string {
	t.Helper()
	for _, env := range os.Environ() {
		name, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(name, "TELL_") {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	base := t.TempDir()
	t.Chdir(base)
	t.Setenv("HOME", base)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config"))
	system := filepath.Join(base, "system.yaml")
	if err := os.WriteFile(system, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELL_SYSTEM_CONFIG", system)
	return base
}

func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), append([]string{"tell"}, args...), config.BuildOpts{Version: "test"}, &out, &errOut)
	return code, out.String(), errOut.String()
}

// Invocation precedence must not send a scheduled message to the wrong tag;
// flags override env even if the overridden env value is invalid.
func TestCLIFlagEnvironmentPrecedence(t *testing.T) {
	base := environment(t)
	if err := os.WriteFile(filepath.Join(base, "tell.yaml"), []byte("timeout: 3s\napprise: {tag: file}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELL_TIMEOUT", "4s")
	t.Setenv("TELL_APPRISE_TAG", "environment")
	code, out, errOut := run("config", "--json")
	if code != 0 || errOut != "" {
		t.Fatalf("%d: %s %s", code, out, errOut)
	}
	var result struct {
		Config config.Config `json:"config"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Config.Timeout != "4s" || result.Config.Apprise.Tag != "environment" || result.Config.Origins["apprise.tag"] != "env:TELL_APPRISE_TAG" {
		t.Fatalf("env precedence: %s", out)
	}
	t.Setenv("TELL_TIMEOUT", "invalid-secret-sentinel")
	code, out, errOut = run("--json", "config", "check", "--timeout", "6s", "--apprise-tag", "flag")
	if code != 0 || errOut != "" {
		t.Fatalf("flag override: %d %s %s", code, out, errOut)
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Config.Timeout != "6s" || result.Config.Apprise.Tag != "flag" || result.Config.Origins["timeout"] != "flag:--timeout" {
		t.Fatalf("flag precedence: %s", out)
	}
}

// Errors must remain parseable JSON without leaking invalid setting values.
// A value literally named --json must not change the output protocol.
func TestCLIErrorOutput(t *testing.T) {
	environment(t)
	t.Setenv("TELL_TIMEOUT", "invalid-secret-sentinel")
	code, out, errOut := run("--json", "config", "check")
	if code != 1 || errOut != "" || !json.Valid([]byte(out)) || strings.Contains(out, "secret-sentinel") {
		t.Fatalf("bad config error: %d %s %s", code, out, errOut)
	}
	code, out, errOut = run("--unknown", "--json")
	if code != 2 || errOut != "" || !json.Valid([]byte(out)) {
		t.Fatalf("bad usage error: %d %s %s", code, out, errOut)
	}
	code, out, errOut = run("--apprise-tag", "--json", "--unknown")
	if code != 2 || out != "" || errOut == "" {
		t.Fatalf("value mistaken for output flag: %d %s %s", code, out, errOut)
	}
}

// Inspecting a repo must not create its config, and init's CLI plumbing must
// preserve an existing file rather than accepting overridden defaults as input.
func TestCLIInspectionAndInit(t *testing.T) {
	base := environment(t)
	code, out, errOut := run("--json")
	if code != 0 || errOut != "" || !json.Valid([]byte(out)) {
		t.Fatalf("inspect: %d %s %s", code, out, errOut)
	}
	path := filepath.Join(base, ".tell.yaml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("inspection created config")
	}
	code, out, errOut = run("init", "--local", "--json")
	if code != 0 || !strings.Contains(out, `"created":true`) {
		t.Fatalf("init: %d %s %s", code, out, errOut)
	}
	if err := os.WriteFile(path, []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = run("init", "--local", "--json")
	if code != 0 || !strings.Contains(out, `"created":false`) {
		t.Fatalf("repeat init: %d %s %s", code, out, errOut)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "sentinel\n" {
		t.Fatal("init overwrote existing config")
	}
}
