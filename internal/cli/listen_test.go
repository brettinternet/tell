package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	configuration := "listen:\n  slack: {app_token_env: TELL_TEST_SLACK_SECRET}\n  routes:\n    - name: only-route\n      source: slack\n      type: channel_created\n      match: {event.channel.name: {prefix: inc-}}\n      run: [" + command + "]\n"
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	event := listen.Envelope{Source: "slack", Type: "channel_created", ID: "Ev1", Data: map[string]json.RawMessage{"event": json.RawMessage(`{"channel":{"name":"inc-api"}}`)}}
	var inspection, inspectionError bytes.Buffer
	if code := Run(context.Background(), []string{"tell", "--config", file, "--json", "listen"}, config.BuildOpts{}, &inspection, &inspectionError); code != 0 || !strings.Contains(inspection.String(), "only-route") || strings.Contains(inspection.String(), command) {
		t.Fatalf("inspection leaked argv or omitted route name: code=%d %s %s", code, inspection.String(), inspectionError.String())
	}

	run := func(args []string) ([][3]string, error) {
		var out, diag bytes.Buffer
		var reports [][3]string
		root := NewRootCommand(config.BuildOpts{}, &out, &diag)
		root.Command("listen").Action = func(ctx context.Context, cmd *urfave.Command) error {
			return runListen(ctx, cmd, func(_ context.Context, opts listen.SlackOptions) error {
				if opts.Token != "fixture-token" || opts.Timeout <= 0 {
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
				var reportOnce sync.Once
				opts.OnEvent(event, func(route, outcome, reason string) {
					reports = append(reports, [3]string{route, outcome, reason})
					reportOnce.Do(func() { close(reported) })
				})
				if opts.OnStop != nil {
					<-reported
					opts.OnStop()
				}
				if len(reports) == 0 {
					t.Error("no route receipt")
				}
				return nil
			})
		}
		err := root.Run(context.Background(), append([]string{"tell", "--config", file, "listen"}, args...))
		return reports, err
	}

	reports, err := run([]string{"--dry-run", "slack"})
	if err != nil || !reflect.DeepEqual(reports, [][3]string{{"only-route", "dry_run", ""}}) {
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
	if err != nil || !reflect.DeepEqual(reports, [][3]string{{"only-route", "accepted", ""}}) {
		t.Fatalf("dispatch receipts: %v %v", reports, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("dispatch did not execute a matched route")
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
		return runListen(ctx, cmd, func(ctx context.Context, opts listen.SlackOptions) error {
			if opts.OnEvent == nil {
				t.Fatal("dispatch handler is missing")
			}
			event := listen.Envelope{Source: "slack", Type: "channel_created", ID: "running", Data: map[string]json.RawMessage{}}
			opts.OnEvent(event, func(route, outcome, reason string) { results <- [3]string{route, outcome, reason} })
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
		{[]string{"listen", "--print"}, 2}, {[]string{"listen", "slack", "slack"}, 2}, {[]string{"listen", "github"}, 2},
		{[]string{"listen", "--all", "slack"}, 2}, {[]string{"--json", "listen", "--print", "slack"}, 2}, {[]string{"listen", "--dry-run", "slack"}, 1}, {[]string{"listen", "--print", "--dry-run", "slack"}, 2},
		{[]string{"listen", "slack"}, 1}, {[]string{"listen", "--print", "slack"}, 1}, {[]string{"listen", "--all", "--print"}, 1},
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
			return runListen(ctx, cmd, func(_ context.Context, opts listen.SlackOptions) error {
				called = true
				if opts.Token != "sentinel-private-app-credential" || opts.Timeout <= 0 {
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
		return runListen(ctx, cmd, func(context.Context, listen.SlackOptions) error { return errors.New("listener output failed") })
	}
	if err := root.Run(context.Background(), []string{"tell", "--config", file, "listen", "--print", "slack"}); err == nil {
		t.Fatal("swallowed output error")
	}
}
