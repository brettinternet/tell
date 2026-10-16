//go:build smoke

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// The binary must preserve stdout and consume stdin only when selected.
	for _, fromStdin := range []bool{false, true} {
		message := "literal notification"
		input := "hook summary"
		if fromStdin {
			message = "-"
		}
		command := exec.Command(binary, "--config", explicit, "--via", "stderr", "notify", message)
		command.Dir = home
		command.Env = []string{}
		command.Stdin = strings.NewReader(input)
		var out, errOut bytes.Buffer
		command.Stdout, command.Stderr = &out, &errOut
		if err := command.Run(); err != nil {
			t.Fatal(err)
		}
		want := message + "\n"
		if fromStdin {
			want = input + "\n"
		}
		if out.Len() != 0 || errOut.String() != want {
			t.Fatalf("notification: %q %q", out.String(), errOut.String())
		}
	}
	output, code := invoke("--json", "--timeout", "0s", "config", "check")
	if code != 1 || !json.Valid([]byte(output)) || !strings.Contains(output, "positive") {
		t.Fatalf("invalid config: %d %s", code, output)
	}
	// Service environments may lack HOME; an explicit file needs no user config.
	output, code = run([]string{}, "--config", explicit, "config", "check")
	if code != 0 {
		t.Fatalf("explicit config without HOME: %d %s", code, output)
	}
}

// Catch repeated-alert spam, accidental forwarding of hook JSON/log secrets,
// and retries or failed hooks caused by a failed notification.
func TestNoonHook(t *testing.T) {
	binary, err := filepath.Abs("../bin/tell")
	if err != nil {
		t.Fatal(err)
	}
	hook, err := filepath.Abs("../examples/noon/notify.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, previous, status string
		send, providerFails    bool
	}{
		{name: "first failure", previous: "succeeded", status: "failed", send: true},
		{name: "timeout", previous: "succeeded", status: "timed_out", send: true},
		{name: "repeat failure", previous: "failed", status: "failed"},
		{name: "same failure episode", previous: "failed", status: "timed_out"},
		{name: "recovery", previous: "timed_out", status: "succeeded", send: true},
		{name: "repeat success", previous: "succeeded", status: "succeeded"},
		{name: "no baseline", status: "failed"},
		{name: "unknown predecessor", previous: "unknown", status: "failed"},
		{name: "provider failure", previous: "succeeded", status: "failed", send: true, providerFails: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(root, "project with spaces")
			bin := filepath.Join(root, "bin")
			config := filepath.Join(root, "config", "tell")
			for _, dir := range []string{project, bin, config} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(binary, filepath.Join(bin, "tell")); err != nil {
				t.Fatal(err)
			}
			provider := filepath.Join(bin, "provider")
			write(provider, `#!/bin/sh
printf '%s\n' "$@" > "$CAPTURE.destination"
printf '%s\n' "$TELL_PROJECT" "$TELL_TYPE" > "$CAPTURE.metadata"
printf 'attempt\n' >> "$CAPTURE.attempts"
/bin/cat > "$CAPTURE.body"
exit "$PROVIDER_EXIT"
`)
			write(filepath.Join(config, "config.yaml"), fmt.Sprintf("version: 1\nnotify:\n  via: command\n  command: [%q, fixture-inbox]\n", provider))
			const job = "checks; literal"
			run := func(id int, status string) map[string]any {
				return map[string]any{"id": id, "job": job, "project": project, "status": status}
			}
			write(filepath.Join(root, "job.log"), "PRIVATE LOG MUST NOT BE SENT")
			code := "0"
			if tt.providerFails {
				code = "7"
			}
			command := exec.Command("/bin/sh", hook)
			command.Dir = project
			command.Env = []string{
				"HOME=" + root, "XDG_CONFIG_HOME=" + filepath.Dir(config),
				"PATH=" + bin + ":/usr/bin:/bin",
				"NOON_PROJECT=" + project, "NOON_JOB=" + job,
				"NOON_RUN_ID=42", "NOON_STATUS=" + tt.status,
				"NOON_EXIT_CODE=0", "NOON_LOG_FILE=" + filepath.Join(root, "job.log"),
				"CAPTURE=" + filepath.Join(root, "capture"), "PROVIDER_EXIT=" + code,
			}
			var previous any
			if tt.previous != "" {
				command.Env = append(command.Env, "NOON_PREVIOUS_STATUS="+tt.previous)
				previous = tt.previous
			}
			summary, err := json.Marshal(map[string]any{
				"schema_version": 1, "previous_status": previous, "project": map[string]string{"path": project},
				"run": run(42, tt.status), "sentinel": "PRIVATE SUMMARY MUST NOT BE SENT",
			})
			if err != nil {
				t.Fatal(err)
			}
			command.Stdin = bytes.NewReader(summary)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("hook must remain best effort: %v %s", err, output)
			}
			body, err := os.ReadFile(filepath.Join(root, "capture.body"))
			if !tt.send {
				if !os.IsNotExist(err) {
					t.Fatalf("unexpected delivery: %q, %v", body, err)
				}
				return
			}
			want := fmt.Sprintf("Job: %s\nStatus: %s\nRun: 42\nProject: %s\nLogs: run noon logs 42 from the project directory.", job, tt.status, project)
			if err != nil || string(body) != want {
				t.Fatalf("message: %q, %v; want %q", body, err, want)
			}
			kind := "failure"
			if tt.status == "succeeded" {
				kind = "success"
			}
			for suffix, want := range map[string]string{
				"destination": "fixture-inbox\n",
				"metadata":    project + "\n" + kind + "\n",
				"attempts":    "attempt\n",
			} {
				got, err := os.ReadFile(filepath.Join(root, "capture."+suffix))
				if err != nil || string(got) != want {
					t.Fatalf("%s: %q, %v; want %q", suffix, got, err, want)
				}
			}
		})
	}
}
