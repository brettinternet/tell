package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
	"github.com/brettinternet/tell/internal/listen"
	urfave "github.com/urfave/cli/v3"
)

// CLI selectors must never silently initiate a connection; config inspection
// and unrelated commands must work with a missing inbound credential.
func TestListenPreviewAndDispatchIsolation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TELL_TEST_SLACK_SECRET", "fixture-token")
	marker := filepath.Join(dir, "action-ran")
	command := filepath.Join(dir, "route-command")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf ran > "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	configuration := "listen:\n  slack: {app_token_env: TELL_TEST_SLACK_SECRET}\n  routes:\n    - name: only-route\n      source: slack\n      type: channel_created\n      match: {event.channel.name: {prefix: inc-}}\n      run: [" + command + "]\n" +
		"    - name: notify-channel\n      source: slack\n      type: channel_created\n      notify: {message: 'Alert ${data.event.channel.name}', via: stderr}\n" +
		"    - name: send-missing\n      source: slack\n      type: channel_created\n      send: {message: '${data.event.missing}', via: hcom, to: reviewer}\n"
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	event := listen.Envelope{Source: "slack", Type: "channel_created", ID: "Ev1", Data: map[string]json.RawMessage{"event": json.RawMessage(`{"channel":{"name":"inc-api"}}`)}}
	var inspection, inspectionError bytes.Buffer
	if code := Run(context.Background(), []string{"tell", "--config", file, "--json", "listen"}, config.BuildOpts{}, &inspection, &inspectionError); code != 0 || !strings.Contains(inspection.String(), "only-route") || strings.Contains(inspection.String(), command) || strings.Contains(inspection.String(), "Alert ${") {
		t.Fatalf("inspection leaked argv or omitted route name: code=%d %s %s", code, inspection.String(), inspectionError.String())
	}

	run := func(args []string) ([][3]string, error) {
		var out, diag bytes.Buffer
		var reports [][3]string
		var reportsMu sync.Mutex
		root := NewRootCommand(config.BuildOpts{}, &out, &diag)
		root.Command("listen").Action = func(ctx context.Context, cmd *urfave.Command) error {
			return runListen(ctx, cmd, func(_ context.Context, opts listen.CaptureOptions) error {
				if len(opts.Sources) != 1 || opts.Sources[0].Token != "fixture-token" || opts.Sources[0].Timeout <= 0 {
					t.Error("selected listener options are wrong")
				}
				if cmd.Bool("print") {
					if opts.OnEvent != nil {
						t.Error("print mode must not evaluate routes")
					}
					return nil
				}
				if opts.OnEvent == nil {
					t.Fatal("dispatch/preview did not install a handler")
				}
				reported := make(chan struct{})
				opts.OnEvent(event, func(route, outcome, reason string) {
					reportsMu.Lock()
					reports = append(reports, [3]string{route, outcome, reason})
					complete := len(reports) == 3
					reportsMu.Unlock()
					if complete {
						close(reported)
					}
				}, io.Discard)
				if opts.OnStop != nil {
					<-reported
					opts.OnStop()
				}
				reportsMu.Lock()
				hasReports := len(reports) > 0
				reportsMu.Unlock()
				if !hasReports {
					t.Error("no route receipt")
				}
				return nil
			})
		}
		err := root.Run(context.Background(), append([]string{"tell", "--config", file, "listen"}, args...))
		reportsMu.Lock()
		result := append([][3]string(nil), reports...)
		reportsMu.Unlock()
		sort.Slice(result, func(i, j int) bool { return result[i][0] < result[j][0] })
		return result, err
	}

	reports, err := run([]string{"--dry-run", "slack"})
	if err != nil || !reflect.DeepEqual(reports, [][3]string{{"notify-channel", "dry_run", ""}, {"only-route", "dry_run", ""}, {"send-missing", "not_sent", "template_value"}}) {
		t.Fatalf("dry-run route receipts: %v %v", reports, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("dry-run executed the route")
	}
	reports, err = run([]string{"--print", "slack"})
	if err != nil || len(reports) != 0 {
		t.Fatalf("print mode matched routes: %v %v", reports, err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("print mode executed the route")
	}
	reports, err = run([]string{"slack"})
	if err != nil || !reflect.DeepEqual(reports, [][3]string{{"notify-channel", "accepted", ""}, {"only-route", "accepted", ""}, {"send-missing", "not_sent", "template_value"}}) {
		t.Fatalf("dispatch receipts: %v %v", reports, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("dispatch did not execute a matched route")
	}
}

// Offline route testing must reuse admission and dry-run receipts without
// credentials, connections, or actions, and never echo envelope contents.
func TestListenDryRunEvents(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TELL_TEST_SLACK_SECRET", "")
	marker := filepath.Join(dir, "action-ran")
	command := filepath.Join(dir, "route-command")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf ran > "+marker+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	configuration := "listen:\n  slack: {app_token_env: TELL_TEST_SLACK_SECRET}\n  routes:\n" +
		"    - name: incident\n      source: slack\n      type: channel_created\n      match: {event.channel.name: {prefix: inc-}}\n      run: [" + command + "]\n" +
		"    - name: notify-channel\n      source: slack\n      type: channel_created\n      notify: {message: 'Alert ${data.event.channel.name}', via: stderr}\n"
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	envelope := func(id, kind, data string, version int) string {
		return `{"schema_version":` + strconv.Itoa(version) + `,"source":"slack","type":"` + kind + `","id":"` + id + `","received_at":"2026-11-17T19:00:00.000Z","data":` + data + "}\n"
	}
	created := envelope("Ev1", "channel_created", `{"event":{"channel":{"name":"inc-sentinel-body"}}}`, 1)
	events := filepath.Join(dir, "events.ndjson")
	input := created + created + "{sentinel-body\n\n" +
		envelope("Ev2", "reaction_added", `{"event":{"reaction":"sentinel-body"}}`, 1) +
		envelope("Ev3", "channel_created", `{"sentinel-body":true}`, 2) +
		envelope("Ev4", "channel_created", `{"pad":"sentinel-body`+strings.Repeat("x", 256<<10)+`"}`, 1)
	if err := os.WriteFile(events, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diag bytes.Buffer
	code := Run(context.Background(), []string{"tell", "--config", file, "listen", "--dry-run", "--events", events}, config.BuildOpts{}, &out, &diag)
	got := strings.Split(strings.TrimSpace(diag.String()), "\n")
	sort.Strings(got)
	want := []string{
		`tell listen: source=file outcome=not_sent/event_size line=7`,
		`tell listen: source=file outcome=not_sent/invalid_envelope line=6`,
		`tell listen: source=file outcome=not_sent/invalid_json line=3`,
		`tell listen: source=slack id="Ev1" type="channel_created" outcome=accepted`,
		`tell listen: source=slack id="Ev1" type="channel_created" outcome=duplicate`,
		`tell listen: source=slack id="Ev1" type="channel_created" route="incident" outcome=dry_run`,
		`tell listen: source=slack id="Ev1" type="channel_created" route="notify-channel" outcome=dry_run`,
		`tell listen: source=slack id="Ev2" type="reaction_added" outcome=accepted`,
		`tell listen: source=slack id="Ev2" type="reaction_added" route="" outcome=no_match`,
	}
	if code != 1 || out.Len() != 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("code=%d out=%q receipts:\n%s", code, out.String(), strings.Join(got, "\n"))
	}
	if strings.Contains(diag.String(), "sentinel") || strings.Contains(diag.String(), command) || strings.Contains(diag.String(), "Alert") {
		t.Fatal("receipt leaked envelope data, rendered text, or argv")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("events dry-run executed the route")
	}
	out.Reset()
	diag.Reset()
	if code := Run(context.Background(), []string{"tell", "--config", file, "listen", "--dry-run", "--events", events + ".missing"}, config.BuildOpts{}, &out, &diag); code != 1 {
		t.Fatalf("unreadable events file exit = %d", code)
	}
}

func TestListenDryRunHonorsContextCancellation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TELL_TEST_SLACK_SECRET", "fixture-token")
	file := filepath.Join(dir, "config.yaml")
	configuration := `listen:
  slack: {app_token_env: TELL_TEST_SLACK_SECRET}
  routes:
    - name: dry-run
      source: slack
      type: event
      send: {message: hi, via: hcom, to: reviewer}
`
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out, diag bytes.Buffer
	root := NewRootCommand(config.BuildOpts{}, &out, &diag)
	var result [3]string
	root.Command("listen").Action = func(ctx context.Context, cmd *urfave.Command) error {
		return runListen(ctx, cmd, func(_ context.Context, opts listen.CaptureOptions) error {
			opts.OnEvent(listen.Envelope{Source: "slack", Type: "event", ID: "Ev1"}, func(route, outcome, reason string) {
				result = [3]string{route, outcome, reason}
			}, io.Discard)
			return nil
		})
	}
	if err := root.Run(ctx, []string{"tell", "--config", file, "listen", "--dry-run", "slack"}); err != nil {
		t.Fatal(err)
	}
	if result != [3]string{"dry-run", "not_sent", "cancelled"} {
		t.Fatalf("cancelled dry-run result = %v", result)
	}
}

func TestListenerActionFixtureProcess(t *testing.T) {
	if !strings.Contains(strings.Join(os.Args, " "), "--fixture-action-child") {
		return
	}
	started := os.Getenv("FIXTURE_ACTION_STARTED")
	release := os.Getenv("FIXTURE_ACTION_RELEASE")
	if err := os.WriteFile(started, nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(release); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("action fixture was not released")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSecondSignalCancelsRunningAction(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TELL_TEST_SLACK_SECRET", "fixture-token")
	started := filepath.Join(dir, "started")
	release := filepath.Join(dir, "release")
	t.Setenv("FIXTURE_ACTION_STARTED", started)
	t.Setenv("FIXTURE_ACTION_RELEASE", release)
	file := filepath.Join(dir, "config.yaml")
	configuration := "listen:\n  slack: {app_token_env: TELL_TEST_SLACK_SECRET}\n  routes:\n    - name: cancel-me\n      source: slack\n      type: channel_created\n      run: [" + strconv.Quote(os.Args[0]) + ", '-test.run=^TestListenerActionFixtureProcess$', --, --fixture-action-child]\n"
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	second := make(chan os.Signal, 1)
	ctx = WithSecondSignal(ctx, second)
	var out, diag bytes.Buffer
	root := NewRootCommand(config.BuildOpts{}, &out, &diag)
	results := make(chan [3]string, 1)
	root.Command("listen").Action = func(ctx context.Context, cmd *urfave.Command) error {
		return runListen(ctx, cmd, func(ctx context.Context, opts listen.CaptureOptions) error {
			if opts.OnEvent == nil {
				t.Fatal("dispatch handler is missing")
			}
			event := listen.Envelope{Source: "slack", Type: "channel_created", ID: "running", Data: map[string]json.RawMessage{}}
			opts.OnEvent(event, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} }, io.Discard)
			waitForFile(t, started)
			go func() { <-ctx.Done(); second <- syscall.SIGTERM }()
			cancel()
			<-ctx.Done()
			opts.OnStop()
			return nil
		})
	}
	err := root.Run(ctx, []string{"tell", "--config", file, "listen", "slack"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-results:
		if result != [3]string{"cancel-me", "uncertain", "cancelled"} {
			t.Fatalf("second signal action receipt: %v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second signal did not cancel the action")
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for action start")
}

func TestListenCLISelection(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("TELL_TEST_SLACK_SECRET", "")
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("listen:\n  slack:\n    app_token_env: TELL_TEST_SLACK_SECRET\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"listen"}, 0}, {[]string{"--json", "listen"}, 0}, {[]string{"config", "check"}, 0}, {[]string{"--json", "config"}, 0}, {[]string{"doctor"}, 0},
		{[]string{"listen", "--print"}, 2}, {[]string{"listen", "slack", "slack"}, 2}, {[]string{"listen", "github"}, 1},
		{[]string{"listen", "--all", "slack"}, 2}, {[]string{"--json", "listen", "--print", "slack"}, 2}, {[]string{"listen", "--dry-run", "slack"}, 1}, {[]string{"listen", "--print", "--dry-run", "slack"}, 2},
		{[]string{"listen", "slack"}, 1}, {[]string{"listen", "--print", "slack"}, 1}, {[]string{"listen", "--all", "--print"}, 1},
		{[]string{"listen", "--events", "-"}, 2}, {[]string{"listen", "--dry-run", "--events", "-", "slack"}, 2}, {[]string{"listen", "--dry-run", "--events", "-", "--all"}, 2}, {[]string{"listen", "--print", "--events", "-"}, 2},
	} {
		var out, diag bytes.Buffer
		code := Run(context.Background(), append([]string{"tell", "--config", file}, tc.args...), config.BuildOpts{}, &out, &diag)
		if code != tc.code {
			t.Errorf("%v: code=%d want=%d %s %s", tc.args, code, tc.code, out.String(), diag.String())
		}
	}
	t.Setenv("TELL_TEST_SLACK_SECRET", "sentinel-private-app-credential")
	for _, args := range [][]string{{"listen", "--print", "slack"}, {"listen", "--print", "--all"}} {
		var out, diag bytes.Buffer
		called := false
		root := NewRootCommand(config.BuildOpts{}, &out, &diag)
		root.Command("listen").Action = func(ctx context.Context, cmd *urfave.Command) error {
			return runListen(ctx, cmd, func(_ context.Context, opts listen.CaptureOptions) error {
				called = true
				if len(opts.Sources) != 1 || opts.Sources[0].Token != "sentinel-private-app-credential" || opts.Sources[0].Timeout <= 0 {
					t.Error("wrong selected options")
				}
				return nil
			})
		}
		if err := root.Run(context.Background(), append([]string{"tell", "--config", file}, args...)); err != nil || !called {
			t.Fatalf("%v %v", args, err)
		}
		if strings.Contains(out.String()+diag.String(), "sentinel-private") {
			t.Fatal("credential leaked")
		}
	}
	// Capture failures remain exit 1, not delivery receipts or successful stubs.
	var out, diag bytes.Buffer
	root := NewRootCommand(config.BuildOpts{}, &out, &diag)
	root.Command("listen").Action = func(ctx context.Context, cmd *urfave.Command) error {
		return runListen(ctx, cmd, func(context.Context, listen.CaptureOptions) error { return errors.New("listener output failed") })
	}
	if err := root.Run(context.Background(), []string{"tell", "--config", file, "listen", "--print", "slack"}); err == nil {
		t.Fatal("swallowed output error")
	}
}
