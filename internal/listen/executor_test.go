package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

type commandRecord struct {
	Args        []string          `json:"args"`
	CWD         string            `json:"cwd"`
	Stdin       string            `json:"stdin"`
	EOF         bool              `json:"eof"`
	Environment map[string]string `json:"environment"`
	Credential  bool              `json:"credential"`
}

// This test is also the real subprocess fixture used by executor tests. It
// catches any future use of a shell or a failure to close the event stdin.
func TestCommandFixtureProcess(t *testing.T) {
	if !contains(os.Args, "--fixture-child") {
		return
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	if recordDir := os.Getenv("FIXTURE_RECORD_DIR"); recordDir != "" {
		file, err := os.OpenFile(filepath.Join(recordDir, os.Getenv("TELL_ROUTE")), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString("called\\n"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if recordPath := os.Getenv("FIXTURE_RECORD"); recordPath != "" {
		environment := map[string]string{}
		for _, key := range []string{"TELL_EVENT_SOURCE", "TELL_EVENT_TYPE", "TELL_EVENT_ID", "TELL_ROUTE", "TELL_PROJECT"} {
			environment[key] = os.Getenv(key)
		}
		_, credential := os.LookupEnv("LISTENER_CREDENTIAL")
		record := commandRecord{Args: os.Args, CWD: currentDirectory(t), Stdin: string(stdin), EOF: true, Environment: environment, Credential: credential}
		encoded, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(recordPath, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if startDir := os.Getenv("FIXTURE_START_DIR"); startDir != "" {
		if err := os.WriteFile(filepath.Join(startDir, os.Getenv("TELL_EVENT_ID")+"-"+os.Getenv("TELL_ROUTE")), nil, 0600); err != nil {
			t.Fatal(err)
		}
		release := os.Getenv("FIXTURE_RELEASE_FILE")
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(release); err == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("fixture was not released")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	_, _ = os.Stdout.WriteString("CHILD-STDOUT-SENTINEL\n")
	_, _ = os.Stderr.WriteString("CHILD-STDERR-SENTINEL\n")
	for _, argument := range os.Args {
		if code, ok := strings.CutPrefix(argument, "--fixture-exit="); ok {
			exit, err := strconv.Atoi(code)
			if err != nil {
				t.Fatal("invalid fixture exit code")
			}
			os.Exit(exit)
		}
	}
}

func TestExecutorCommandIsolationAndLiteralArgv(t *testing.T) {
	t.Setenv("LISTENER_CREDENTIAL", "private-inbound-token")
	t.Setenv("TELL_EVENT_SOURCE", "wrong-source")
	t.Setenv("TELL_EVENT_TYPE", "wrong-type")
	t.Setenv("TELL_EVENT_ID", "wrong-id")
	t.Setenv("TELL_ROUTE", "wrong-route")
	t.Setenv("TELL_PROJECT", "wrong-project")
	recordPath := filepath.Join(t.TempDir(), "record.json")
	t.Setenv("FIXTURE_RECORD", recordPath)
	marker := filepath.Join(t.TempDir(), "shell-was-used")
	literal := "$(touch " + marker + "); *"
	argv := []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child", "two words", "", literal}
	cfg := config.Config{
		Project: "/project/selected-by-tell", Timeout: "3s",
		Listen: config.Listen{Slack: &config.Slack{AppTokenEnv: "LISTENER_CREDENTIAL"}},
	}
	route := config.Route{Name: "safe-run", Source: "slack", Type: "event", Run: argv}
	event := Envelope{SchemaVersion: 1, Source: "slack", Type: "event", ID: "Ev-1", ReceivedAt: "2026-10-17T20:59:00.123Z", Data: map[string]json.RawMessage{"text": json.RawMessage(`"private-body"`)}}
	cwd := currentDirectory(t)
	results := make(chan [3]string, 1)
	executor := NewExecutor(cfg, cwd)
	if !executor.Submit(route, event, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }) {
		t.Fatal("action was not admitted")
	}
	select {
	case result := <-results:
		if result != [3]string{"safe-run", "accepted", ""} {
			t.Fatalf("unexpected action receipt: %v", result)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("subprocess did not finish")
	}
	executor.Stop()
	executor.Wait()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("shell syntax was evaluated: %v", err)
	}
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record commandRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	markerIndex := indexOf(record.Args, "--fixture-child")
	if markerIndex < 0 || !reflect.DeepEqual(record.Args[markerIndex+1:], []string{"two words", "", literal}) {
		t.Fatalf("argv was not passed literally: %q", record.Args)
	}
	if record.CWD != cwd || !record.EOF {
		t.Fatalf("wrong working directory or stdin EOF: %+v", record)
	}
	var gotEvent Envelope
	if err := json.Unmarshal([]byte(record.Stdin), &gotEvent); err != nil || gotEvent.ID != event.ID || gotEvent.Data["text"] == nil {
		t.Fatalf("stdin did not contain the event envelope: %q %v", record.Stdin, err)
	}
	wantEnv := map[string]string{
		"TELL_EVENT_SOURCE": "slack", "TELL_EVENT_TYPE": "event", "TELL_EVENT_ID": "Ev-1",
		"TELL_ROUTE": "safe-run", "TELL_PROJECT": "/project/selected-by-tell",
	}
	if !reflect.DeepEqual(record.Environment, wantEnv) || record.Credential {
		t.Fatalf("wrong child environment or leaked credential: %+v", record)
	}
	if strings.Contains(strings.Join(record.Args, " "), "CHILD-STDOUT-SENTINEL") {
		t.Fatal("child output appeared in argv receipt")
	}
}

func TestNativeNotifyInheritsCascadeAndDryRunDoesNotProbe(t *testing.T) {
	providerDir := t.TempDir()
	t.Setenv("PATH", providerDir)
	t.Setenv("APPRISE_URLS", "")
	record := filepath.Join(t.TempDir(), "provider-called")
	if err := os.WriteFile(filepath.Join(providerDir, "apprise"), []byte("#!/bin/sh\nprintf called > "+record+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Project: "/project", Timeout: "1s", Secrets: config.Secrets{Policy: "redact"},
		Notify: config.Notify{Via: "apprise", Steps: []config.NotifyStep{{Via: "apprise"}, {Via: "stderr"}}},
	}
	route := config.Route{Name: "cascade", Notify: &config.NotifyAction{Message: "listener cascade"}}
	event := Envelope{Source: "slack", Type: "event", ID: "event-1", Data: map[string]json.RawMessage{}}
	if outcome, reason := ValidateAction(context.Background(), cfg, route, event); outcome != "dry_run" || reason != "" {
		t.Fatalf("dry run: %s %s", outcome, reason)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("dry run probed the provider")
	}

	t.Setenv("PATH", t.TempDir())
	var stderr bytes.Buffer
	results := make(chan [3]string, 1)
	executor := NewExecutor(cfg, currentDirectory(t))
	if !executor.SubmitWithWriter(route, event, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }, &stderr) {
		t.Fatal("native cascade was not admitted")
	}
	select {
	case result := <-results:
		if result != [3]string{"cascade", "accepted", ""} {
			t.Fatalf("native cascade receipt: %v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("native cascade did not finish")
	}
	executor.Stop()
	executor.Wait()
	if stderr.String() != "listener cascade\n" {
		t.Fatalf("notify action did not inherit the cascade: %q", stderr.String())
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatal("unavailable first step unexpectedly ran provider")
	}

	static := config.Route{Name: "static", Notify: &config.NotifyAction{Message: "one route", Via: "stderr"}}
	rendered, reason := renderAction(context.Background(), cfg, static, event)
	if reason != "" || len(rendered.cfg.Notify.Steps) != 0 || rendered.cfg.Notify.Via != "stderr" {
		t.Fatalf("static via retained inherited cascade: %+v %s", rendered.cfg.Notify, reason)
	}
}

// Action-local selectors must not alter later events or race between executor workers.
func TestNativeNotifyOverridesDoNotMutateSharedOrigins(t *testing.T) {
	cfg := config.Config{
		Project: "/project", Secrets: config.Secrets{Policy: "redact"},
		Notify:  config.Notify{Via: "apprise", Steps: []config.NotifyStep{{Via: "apprise", Tag: "step-tag"}, {Via: "herdr", Session: "step-session"}}},
		Origins: map[string]string{"apprise.tag": "personal.yaml", "herdr.session": "personal.yaml"},
	}
	wantOrigins := map[string]string{"apprise.tag": "personal.yaml", "herdr.session": "personal.yaml"}
	override := config.Route{Notify: &config.NotifyAction{Message: "hello", AppriseTag: "action-tag", HerdrSession: "action-session"}}
	inherited := config.Route{Notify: &config.NotifyAction{Message: "hello"}}
	check := func() {
		for _, route := range []config.Route{override, inherited} {
			if outcome, reason := ValidateAction(context.Background(), cfg, route, Envelope{}); outcome != "dry_run" || reason != "" {
				t.Errorf("validation: %s %s", outcome, reason)
			}
		}
		steps, err := config.EffectiveNotifySteps(cfg)
		if err != nil || steps[0].Tag != "step-tag" || steps[1].Session != "step-session" {
			t.Errorf("action changed inherited routing: %+v %v", steps, err)
		}
	}
	check()
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(check)
	}
	workers.Wait()
	if !reflect.DeepEqual(cfg.Origins, wantOrigins) {
		t.Fatalf("shared origins mutated: %v", cfg.Origins)
	}
}

func TestExecutorCommandOutcomes(t *testing.T) {
	for exit, want := range map[int][2]string{3: {"not_sent", "no_match"}, 9: {"failed", "provider_failure"}} {
		t.Run("exit-"+strconv.Itoa(exit), func(t *testing.T) {
			cfg := config.Config{Project: "/project", Timeout: "3s"}
			route := config.Route{Name: "exit", Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child", "--fixture-exit=" + strconv.Itoa(exit)}}
			result := runOneAction(t, cfg, route)
			if result[1] != want[0] || result[2] != want[1] {
				t.Fatalf("exit %d outcome = %v, want %v", exit, result, want)
			}
		})
	}

	missing := config.Route{Name: "missing", Run: []string{filepath.Join(t.TempDir(), "missing-executable")}}
	if result := runOneAction(t, config.Config{Timeout: "1s"}, missing); result[1] != "not_sent" || result[2] != "missing_executable" {
		t.Fatalf("missing executable outcome: %v", result)
	}
	notExecutable := filepath.Join(t.TempDir(), "not-executable")
	if err := os.WriteFile(notExecutable, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if result := runOneAction(t, config.Config{Timeout: "1s"}, config.Route{Name: "start-failed", Run: []string{notExecutable}}); result[1] != "not_sent" || result[2] != "start_failed" {
		t.Fatalf("failed start outcome: %v", result)
	}

	startDir := t.TempDir()
	t.Setenv("FIXTURE_START_DIR", startDir)
	t.Setenv("FIXTURE_RELEASE_FILE", filepath.Join(t.TempDir(), "never-release"))
	blocking := config.Route{Name: "timeout", Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child"}}
	result := runOneAction(t, config.Config{Timeout: "150ms"}, blocking)
	if result[1] != "uncertain" || result[2] != "timeout" {
		t.Fatalf("timeout outcome: %v", result)
	}
}

func TestExecutorSecondSignalCancellation(t *testing.T) {
	startDir := t.TempDir()
	t.Setenv("FIXTURE_START_DIR", startDir)
	t.Setenv("FIXTURE_RELEASE_FILE", filepath.Join(t.TempDir(), "never-release"))
	cfg := config.Config{Timeout: "5s"}
	route := config.Route{Name: "cancel", Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child"}}
	results := make(chan [3]string, 1)
	executor := NewExecutor(cfg, currentDirectory(t))
	if !executor.Submit(route, Envelope{ID: "cancel-event", Data: map[string]json.RawMessage{}}, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }) {
		t.Fatal("action was not admitted")
	}
	waitFor(t, func() bool {
		_, err := os.Stat(filepath.Join(startDir, "cancel-event-cancel"))
		return err == nil
	})
	executor.Stop()
	executor.CancelRunning()
	select {
	case result := <-results:
		if result[1] != "uncertain" || result[2] != "cancelled" {
			t.Fatalf("second-signal outcome: %v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled subprocess did not finish")
	}
	executor.Wait()
}

func TestExecutorGracefulStopDropsWaitingActions(t *testing.T) {
	startDir := t.TempDir()
	release := filepath.Join(t.TempDir(), "release")
	t.Setenv("FIXTURE_START_DIR", startDir)
	t.Setenv("FIXTURE_RELEASE_FILE", release)
	cfg := config.Config{Timeout: "5s"}
	route := config.Route{Name: "graceful", Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child"}}
	executor := NewExecutor(cfg, currentDirectory(t))
	results := make(chan [3]string, actionWorkers+actionQueueLimit)
	submit := func(id string) bool {
		return executor.Submit(route, Envelope{ID: id, Data: map[string]json.RawMessage{}}, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} })
	}
	for i := 0; i < actionWorkers; i++ {
		if !submit("drain-" + strconv.Itoa(i)) {
			t.Fatal("running action was refused")
		}
	}
	waitFor(t, func() bool {
		entries, err := os.ReadDir(startDir)
		return err == nil && len(entries) == actionWorkers
	})
	native := config.Route{Name: "native-drop", Notify: &config.NotifyAction{Message: "queued", Via: "stderr"}}
	for i := 0; i < actionQueueLimit; i++ {
		id := "drop-" + strconv.Itoa(i)
		var admitted bool
		if i%2 == 0 {
			admitted = executor.SubmitWithWriter(native, Envelope{Source: "slack", Type: "event", ID: id, Data: map[string]json.RawMessage{}}, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }, io.Discard)
		} else {
			admitted = submit(id)
		}
		if !admitted {
			t.Fatalf("waiting action %d was refused", i)
		}
	}
	executor.Stop()
	for i := 0; i < actionQueueLimit; i++ {
		select {
		case result := <-results:
			if result[1] != "not_sent" || result[2] != "shutdown" {
				t.Fatalf("queued shutdown receipt: %v", result)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("queued actions were not reported as dropped")
		}
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < actionWorkers; i++ {
		select {
		case result := <-results:
			if result[1] != "accepted" {
				t.Fatalf("gracefully drained action was cancelled: %v", result)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("running actions did not drain")
		}
	}
	executor.Wait()
}

func TestExecutorAdmissionBounds(t *testing.T) {
	startDir := t.TempDir()
	release := filepath.Join(t.TempDir(), "release")
	t.Setenv("FIXTURE_START_DIR", startDir)
	t.Setenv("FIXTURE_RELEASE_FILE", release)
	cfg := config.Config{Project: "/project", Timeout: "5s"}
	route := config.Route{Name: "bounded", Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child"}}
	executor := NewExecutor(cfg, currentDirectory(t))
	results := make(chan [3]string, actionWorkers+actionQueueLimit)
	submit := func(id string) bool {
		event := Envelope{Source: "slack", Type: "event", ID: id, Data: map[string]json.RawMessage{}}
		return executor.Submit(route, event, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} })
	}
	for i := 0; i < actionWorkers; i++ {
		if !submit("running-" + string(rune('a'+i))) {
			t.Fatal("one of four running slots was refused")
		}
	}
	waitFor(t, func() bool {
		entries, err := os.ReadDir(startDir)
		return err == nil && len(entries) == actionWorkers
	})
	native := config.Route{Name: "native-bounded", Notify: &config.NotifyAction{Message: "queued", Via: "stderr"}}
	for i := 0; i < actionQueueLimit; i++ {
		id := "queued-" + string(rune('a'+i))
		var admitted bool
		if i%2 == 0 {
			admitted = executor.SubmitWithWriter(native, Envelope{Source: "slack", Type: "event", ID: id, Data: map[string]json.RawMessage{}}, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }, io.Discard)
		} else {
			admitted = submit(id)
		}
		if !admitted {
			t.Fatalf("waiting slot %d was refused", i)
		}
	}
	if submit("overflow") {
		t.Fatal("accepted more than four running plus 64 waiting actions")
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < actionWorkers+actionQueueLimit; i++ {
		select {
		case result := <-results:
			if result[1] != "accepted" {
				t.Fatalf("unexpected action outcome: %v", result)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("admitted actions did not finish")
		}
	}
	executor.Stop()
	executor.Wait()
}

func runOneAction(t *testing.T, cfg config.Config, route config.Route) [3]string {
	t.Helper()
	results := make(chan [3]string, 1)
	executor := NewExecutor(cfg, currentDirectory(t))
	if !executor.Submit(route, Envelope{Source: "slack", Type: "event", ID: "one", Data: map[string]json.RawMessage{}}, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }) {
		t.Fatal("action was not admitted")
	}
	select {
	case result := <-results:
		executor.Stop()
		executor.Wait()
		return result
	case <-time.After(5 * time.Second):
		executor.Stop()
		executor.CancelRunning()
		executor.Wait()
		t.Fatal("action did not finish")
		return [3]string{}
	}
}

func currentDirectory(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return cwd
}

func contains(values []string, want string) bool { return indexOf(values, want) >= 0 }
func indexOf(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for subprocess fixture")
}
