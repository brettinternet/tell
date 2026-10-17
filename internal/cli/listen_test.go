package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/config"
	"github.com/brettinternet/tell/internal/listen"
	urfave "github.com/urfave/cli/v3"
)

// CLI selectors must never silently initiate a connection; config inspection
// and unrelated commands must work with a missing inbound credential.
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
		{[]string{"listen", "--all", "slack"}, 2}, {[]string{"--json", "listen", "--print", "slack"}, 2}, {[]string{"listen", "--dry-run", "slack"}, 2},
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
