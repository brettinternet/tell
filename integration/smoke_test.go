//go:build smoke

package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the actual binary and process exit codes with no inherited user,
// provider, or terminal-session environment. No notification service is invoked.
func TestBinary(t *testing.T) {
	binary, err := filepath.Abs("../bin/tell")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	explicit := filepath.Join(home, "explicit.yaml")
	if err := os.WriteFile(explicit, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run := func(env []string, args ...string) (string, int) {
		t.Helper()
		command := exec.Command(binary, args...)
		command.Dir = home
		command.Env = env
		output, err := command.CombinedOutput()
		if err == nil {
			return string(output), 0
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return string(output), exit.ExitCode()
		}
		t.Fatal(err)
		return "", -1
	}
	invoke := func(args ...string) (string, int) {
		t.Helper()
		return run([]string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "XDG_CONFIG_HOME=" + filepath.Join(home, "config")}, args...)
	}
	for _, args := range [][]string{{"--help"}, {"--version"}, {"--json"}, {"--json", "config", "check"}} {
		output, code := invoke(args...)
		if code != 0 {
			t.Fatalf("%v: %d %s", args, code, output)
		}
		if args[0] == "--json" && !json.Valid([]byte(output)) {
			t.Fatalf("invalid JSON: %s", output)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "tell.yaml")); !os.IsNotExist(err) {
		t.Fatal("inspection wrote config")
	}
	for _, args := range [][]string{{"init", "--json"}, {"init", "--local", "--json"}, {"init", "--user", "--json"}, {"--json", "config", "check"}} {
		output, code := invoke(args...)
		if code != 0 || !json.Valid([]byte(output)) {
			t.Fatalf("%v: %d %s", args, code, output)
		}
	}
	output, code := invoke("notify", "must not send")
	if code != 2 {
		t.Fatalf("unsupported delivery must fail: %d %s", code, output)
	}
	output, code = invoke("--json", "--timeout", "0s", "config", "check")
	if code != 1 || !json.Valid([]byte(output)) || !strings.Contains(output, "positive") {
		t.Fatalf("invalid config: %d %s", code, output)
	}
	// Service environments may lack HOME; an explicit file needs no user config.
	output, code = run([]string{}, "--config", explicit, "config", "check")
	if code != 0 {
		t.Fatalf("explicit config without HOME: %d %s", code, output)
	}
}
