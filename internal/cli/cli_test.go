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
	return base
}

func run(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), append([]string{"tell"}, args...), config.BuildOpts{Version: "test"}, &out, &errOut)
	return code, out.String(), errOut.String()
}

func writeUserConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "tell", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
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

// Destination environment/flag overrides must remain usable after shared-file
// restrictions, without changing flag-over-environment precedence.
func TestMCPAgentTargetsOverrideSources(t *testing.T) {
	environment(t)
	t.Setenv("TELL_MCP_AGENT_TARGETS", "true")
	code, out, errOut := run("config", "--json")
	var result struct {
		Config config.Config `json:"config"`
	}
	if code != 0 || errOut != "" || json.Unmarshal([]byte(out), &result) != nil {
		t.Fatalf("environment override: %d %s %s", code, out, errOut)
	}
	if !result.Config.MCP.AgentTargets || result.Config.Origins["mcp.agent_targets"] != "env:TELL_MCP_AGENT_TARGETS" {
		t.Fatalf("environment setting or origin missing: %s", out)
	}
	userConfig := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "tell", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(userConfig), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userConfig, []byte("mcp: {agent_targets: true}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = run("--mcp-agent-targets=false", "config", "--json")
	if code != 0 || errOut != "" || json.Unmarshal([]byte(out), &result) != nil {
		t.Fatalf("flag override: %d %s %s", code, out, errOut)
	}
	if result.Config.MCP.AgentTargets || result.Config.Origins["mcp.agent_targets"] != "flag:--mcp-agent-targets" {
		t.Fatalf("flag did not override enabled user config: %s", out)
	}
	code, out, errOut = run("mcp", "--mcp-agent-targets=false", "--json")
	if code != 2 || !strings.Contains(out+errOut, "mcp does not support --json") {
		t.Fatalf("MCP flags after command: %d %s %s", code, out, errOut)
	}
}

func TestPersonalRoutingOverrides(t *testing.T) {
	for _, tc := range []struct{ key, env, flag, value string }{
		{"notify.via", "TELL_NOTIFY_VIA", "--via", "apprise"},
		{"apprise.config", "TELL_APPRISE_CONFIG", "--apprise-config", "/personal/provider.yaml"},
		{"herdr.session", "TELL_HERDR_SESSION", "--herdr-session", "personal"},
		{"send.to", "TELL_SEND_TO", "--to", "reviewer"},
		{"send.via", "TELL_SEND_VIA", "--send-via", "hcom"},
		{"send.from", "TELL_SEND_FROM", "--from", "nightly"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			environment(t)
			t.Setenv(tc.env, tc.value)
			for _, flag := range []bool{false, true} {
				args := []string{"--json", "config"}
				source := "env:" + tc.env
				if flag {
					t.Setenv(tc.env, "")
					args = append(args, tc.flag, tc.value)
					source = "flag:" + tc.flag
				}
				code, out, errOut := run(args...)
				var result struct{ Config config.Config }
				if code != 0 || errOut != "" || json.Unmarshal([]byte(out), &result) != nil {
					t.Fatalf("override: %d %s %s", code, out, errOut)
				}
				if result.Config.Values()[tc.key] != tc.value || result.Config.Origins[tc.key] != source {
					t.Fatalf("wrong routing/provenance: %s", out)
				}
			}
		})
	}
}

// User init must honor XDG/HOME, preserve private data and refuse planted links.
func TestCLIUserInit(t *testing.T) {
	for _, xdg := range []bool{false, true} {
		t.Run(map[bool]string{false: "home", true: "xdg"}[xdg], func(t *testing.T) {
			base := environment(t)
			dir := filepath.Join(base, "config")
			if !xdg {
				t.Setenv("XDG_CONFIG_HOME", "")
				dir = filepath.Join(base, ".config")
			}
			path := filepath.Join(dir, "tell", "config.yaml")
			code, out, errOut := run("init", "--user", "--json")
			if code != 0 || !strings.Contains(out, `"created":true`) {
				t.Fatalf("init user: %d %s %s", code, out, errOut)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0o600 {
				t.Fatalf("private file: %v %v", info, err)
			}
			data, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(data), "#   config:") {
				t.Fatalf("personal template: %s %v", data, err)
			}
			if code, out, errOut := run("config", "check"); code != 0 {
				t.Fatalf("generated config invalid: %s %s", out, errOut)
			}
			if err := os.WriteFile(path, []byte("sentinel\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			code, out, errOut = run("init", "--user", "--json")
			data, err = os.ReadFile(path)
			if code != 0 || !strings.Contains(out, `"created":false`) || err != nil || string(data) != "sentinel\n" {
				t.Fatalf("overwritten: %d %s %s %v", code, out, errOut, err)
			}
			target := filepath.Join(base, "saved.yaml")
			if err := os.Rename(path, target); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if code, _, _ := run("init", "--user"); code != 1 {
				t.Fatalf("accepted symlink: %d", code)
			}
			for _, args := range [][]string{{"-C", base, "init", "--user"}, {"init", "--user", "--via", "stderr"}} {
				if code, _, _ := run(args...); code != 2 {
					t.Fatalf("accepted conflicting target: %v", args)
				}
			}
			// Ambient shell/service environment must not block init.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TELL_HERDR_SESSION", "main")
			t.Setenv("TELL_CONFIG", filepath.Join(base, "absent.yaml"))
			t.Setenv("TELL_PROJECT", base)
			if code, out, errOut := run("init", "--user", "--json"); code != 0 || !strings.Contains(out, `"created":true`) {
				t.Fatalf("ambient env blocked init: %d %s %s", code, out, errOut)
			}
		})
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

// Detailed diagnostics must not regress to generic errors or change the JSON contract.
func TestActionableErrors(t *testing.T) {
	environment(t)
	for _, tc := range []struct {
		args                     []string
		message, outcome, reason string
		code                     int
	}{
		{[]string{"notify", "--tittle", "x", "body-sentinel"}, "flag provided but not defined: -tittle; run tell notify --help", "usage", "invalid_invocation", 2},
		{[]string{"send", "--tittle", "x", "body-sentinel"}, "flag provided but not defined: -tittle; run tell send --help", "usage", "invalid_invocation", 2},
		{[]string{"notify", "--title"}, "flag needs an argument: --title; run tell notify --help", "usage", "invalid_invocation", 2},
		{[]string{"send", "--intent"}, "flag needs an argument: --intent; run tell send --help", "usage", "invalid_invocation", 2},
		{[]string{"notify", "--type", "sucess", "body-sentinel"}, "invalid --type \"sucess\"; must be info, success, warning, or failure", "usage", "invalid_type", 2},
		{[]string{"send", "--intent", "req", "body-sentinel"}, "invalid --intent \"req\"; must be inform or request", "usage", "invalid_intent", 2},
		{[]string{"notfy", "body-sentinel"}, "unknown command \"notfy\"; did you mean \"notify\"?; run tell --help", "usage", "", 2},
	} {
		for _, structured := range []bool{false, true} {
			args := append([]string{}, tc.args...)
			if structured {
				args = append([]string{"--json"}, args...)
			}
			code, out, diag := run(args...)
			if code != tc.code || strings.Contains(out+diag, "body-sentinel") {
				t.Fatalf("%v: %d %s %s", args, code, out, diag)
			}
			if !structured {
				if out != "" || !strings.Contains(diag, tc.message) {
					t.Fatalf("%v: %s %s", args, out, diag)
				}
				continue
			}
			var result struct {
				SchemaVersion int                                    `json:"schema_version"`
				Outcome       string                                 `json:"outcome"`
				Error         struct{ Code, Message, Reason string } `json:"error"`
			}
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if result.SchemaVersion != 1 || result.Error.Code != tc.outcome || result.Error.Reason != tc.reason || result.Error.Message != tc.message || (tc.reason != "" && result.Outcome != tc.outcome) {
				t.Fatalf("%v: %s", args, out)
			}
		}
	}
}

// All commands must expose the same safe validation error and source, not values.
func TestConfigurationErrorConsistency(t *testing.T) {
	for _, source := range []string{"flag", "env", "file"} {
		t.Run(source, func(t *testing.T) {
			environment(t)
			prefix := []string{}
			origin := "--timeout"
			switch source {
			case "flag":
				prefix = []string{"--timeout", "invalid-secret-sentinel"}
			case "env":
				t.Setenv("TELL_TIMEOUT", "invalid-secret-sentinel")
				origin = "TELL_TIMEOUT"
			case "file":
				origin = "config " + writeUserConfig(t, "timeout: invalid-secret-sentinel\n") + ":1"
			}
			want := origin + ": timeout must be a positive Go duration, such as 10s"
			for _, command := range [][]string{{"config", "check"}, {"notify", "body-sentinel"}, {"send", "body-sentinel"}, {"doctor"}} {
				for _, structured := range []bool{false, true} {
					args := append(append([]string{}, prefix...), command...)
					if structured {
						args = append([]string{"--json"}, args...)
					}
					code, out, diag := run(args...)
					if code != 1 || strings.Contains(out+diag, "sentinel") {
						t.Fatalf("%v: %d %s %s", args, code, out, diag)
					}
					if !structured {
						if out != "" || !strings.Contains(diag, want) {
							t.Fatalf("%v: %s %s", args, out, diag)
						}
						continue
					}
					var result struct {
						SchemaVersion int                                    `json:"schema_version"`
						Outcome       string                                 `json:"outcome"`
						Error         struct{ Code, Message, Reason string } `json:"error"`
					}
					if err := json.Unmarshal([]byte(out), &result); err != nil {
						t.Fatal(err)
					}
					if result.SchemaVersion != 1 || result.Error.Code != "config" || result.Error.Message != want {
						t.Fatalf("%v: %s", args, out)
					}
					if command[0] == "notify" || command[0] == "send" {
						if result.Outcome != "config" || result.Error.Reason != "invalid_config" {
							t.Fatalf("delivery shape: %s", out)
						}
					}
				}
			}
		})
	}
}

// Echoing rejected option values must not expose credentials to logs.
func TestUsageErrorRedactsCredentials(t *testing.T) {
	environment(t)
	token := "ghp_" + "aB3dE6fG9hJ2kL5mN8pQ1rS4tU7vW0xY3zA6"
	for _, args := range [][]string{
		{"notify", "--type", token, "body-sentinel"},
		{"send", "--intent", token, "body-sentinel"},
		{"notify", "--unknown-" + token, "body-sentinel"},
	} {
		code, out, diag := run(append([]string{"--json"}, args...)...)
		if code != 2 || strings.Contains(out+diag, token) || !strings.Contains(out, "[REDACTED]") {
			t.Fatalf("unsafe diagnostic: %d %s %s", code, out, diag)
		}
	}
}

// Framework parse failures must not echo arbitrary values, even when they
// overlap a positional message; unknown YAML keys must not disclose tokens.
func TestForwardedErrorsPreservePrivacy(t *testing.T) {
	environment(t)
	for _, command := range []string{"notify", "send"} {
		for _, structured := range []bool{false, true} {
			args := []string{command, "--mcp-agent-targets=body-sentinel", "body-sentinel"}
			if structured {
				args = append([]string{"--json"}, args...)
			}
			code, out, diag := run(args...)
			if code != 2 || strings.Contains(out+diag, "body-sentinel") || !strings.Contains(out+diag, "for flag -mcp-agent-targets: parse error") {
				t.Fatalf("parser disclosure: %d %s %s", code, out, diag)
			}
		}
	}
	token := "ghp_" + "aB3dE6fG9hJ2kL5mN8pQ1rS4tU7vW0xY3zA6"
	writeUserConfig(t, token+": x\n")
	for _, command := range [][]string{{"notify", "body-sentinel"}, {"send", "body-sentinel"}, {"doctor"}, {"config", "check"}} {
		for _, structured := range []bool{false, true} {
			args := append([]string{}, command...)
			if structured {
				args = append([]string{"--json"}, args...)
			}
			code, out, diag := run(args...)
			if code != 1 || strings.Contains(out+diag, token) || !strings.Contains(out+diag, "[REDACTED]") || !strings.Contains(out+diag, "unknown field") {
				t.Fatalf("config disclosure: %d %s %s", code, out, diag)
			}
		}
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
	path := filepath.Join(base, "tell.yaml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("inspection created config")
	}
	if code, _, _ := run("init", "--local"); code != 2 {
		t.Fatalf("accepted removed --local flag: %d", code)
	}
	code, out, errOut = run("init", "--json")
	if code != 0 || !strings.Contains(out, `"created":true`) {
		t.Fatalf("init: %d %s %s", code, out, errOut)
	}
	if err := os.WriteFile(path, []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut = run("init", "--json")
	if code != 0 || !strings.Contains(out, `"created":false`) {
		t.Fatalf("repeat init: %d %s %s", code, out, errOut)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "sentinel\n" {
		t.Fatal("init overwrote existing config")
	}
}
