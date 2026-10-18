package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	return config.Input{CWD: project, UserConfigDir: filepath.Join(base, "user")}
}

// Shared files and literal credentials cannot cross the inbound trust boundary.
func TestListenPersonalConfig(t *testing.T) {
	for _, data := range []string{"listen: {}\n", "listen:\n  slack:\n    app_token_env: TEST_SLACK_TOKEN\n", "listen: {routes: []}\n"} {
		input := fixture(t)
		writeFile(t, filepath.Join(input.CWD, "tell.yaml"), data)
		if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "personal") {
			t.Fatalf("shared listen accepted: %v", err)
		}
	}
	for _, data := range []string{
		"listen: {slack: {}}\n",
		"listen: {slack: {app_token_env: 'private-token-value'}}\n",
		"listen: {slack: {app_token_env: TOKEN, api_url: 'http://private'}}\n",
	} {
		input := fixture(t)
		file := filepath.Join(input.CWD, "personal.yaml")
		input.File = &file
		writeFile(t, file, data)
		if _, err := config.Load(input); err == nil || strings.Contains(err.Error(), "private-token-value") {
			t.Fatalf("invalid config or leakage: %v", err)
		}
	}
	input := fixture(t)
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, user, "listen: {slack: {app_token_env: FIRST_TOKEN}}\n")
	// A higher layer merges the source by field, retaining only the winning name.
	writeFile(t, filepath.Join(input.CWD, "..", "worktree-metadata", "tell.yaml"), "listen: {slack: {app_token_env: SECOND_TOKEN}}\n")
	cfg, err := config.Load(input)
	if err != nil || cfg.SlackTokenEnv() != "SECOND_TOKEN" {
		t.Fatalf("%+v %v", cfg, err)
	}
	if got := cfg.ChildEnv([]string{"SECOND_TOKEN=secret", "KEEP=value", "SECOND_TOKEN=again"}); !reflect.DeepEqual(got, []string{"KEEP=value"}) {
		t.Fatalf("credential leak: %q", got)
	}
}

// Routes replace as a whole-list setting and retain scalar types without
// converting large integer predicates through float64.
func TestListenRoutesConfig(t *testing.T) {
	input := fixture(t)
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, user, "listen:\n  slack: {app_token_env: TOKEN}\n  routes:\n    - {name: old-route, source: slack, type: event, run: [/bin/true]}\n")
	local := filepath.Join(input.CWD, "..", "worktree-metadata", "tell.yaml")
	writeFile(t, local, "listen:\n  routes:\n    - name: exact-id\n      source: slack\n      type: pull_request\n      match:\n        repository.id: 1234567890123456789012345678901234567890\n        action: opened\n        event.name: {prefix: inc-}\n      run: [/absolute/wrapper, 'two words']\n")
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Listen.Routes) != 1 || cfg.Listen.Routes[0].Name != "exact-id" || cfg.Values()["listen.routes"] != "1" {
		t.Fatalf("route list was not replaced: %+v", cfg.Listen.Routes)
	}
	if got := cfg.Listen.Routes[0].Match["repository.id"].Value; got != json.Number("1234567890123456789012345678901234567890") {
		t.Fatalf("large scalar lost precision or type: %T %v", got, got)
	}
	if cfg.Listen.Routes[0].Match["event.name"].Prefix == nil || *cfg.Listen.Routes[0].Match["event.name"].Prefix != "inc-" {
		t.Fatalf("prefix predicate not parsed: %+v", cfg.Listen.Routes[0].Match)
	}

	for _, routes := range []string{
		"- {name: dup, source: slack, type: e, run: [/bin/true]}\n  - {name: dup, source: slack, type: e, run: [/bin/true]}\n",
		"- {name: Bad, source: slack, type: e, run: [/bin/true]}\n",
		"- {name: bad-path, source: slack, type: e, match: {'event..id': x}, run: [/bin/true]}\n",
		"- {name: no-run, source: slack, type: e}\n",
		"- {name: empty-run, source: slack, type: e, run: []}\n",
		"- {name: blank-executable, source: slack, type: e, run: ['   ']}\n",
		"- {name: typed-argv, source: slack, type: e, run: [true]}\n",
		"- {name: nul-argv, source: slack, type: e, run: [\"bad\\0arg\"]}\n",
		"- {name: null-match, source: slack, type: e, match: {id: null}, run: [/bin/true]}\n",
		"- {name: bad-prefix, source: slack, type: e, match: {id: {prefix: 1}}, run: [/bin/true]}\n",
		"- {name: bad-action, source: slack, type: e, notify: {message: body, via: invalid}}\n",
		"- {name: bad-source, source: github, type: e, run: [/bin/true]}\n",
	} {
		file := filepath.Join(input.CWD, "routes.yaml")
		writeFile(t, file, "listen:\n  slack: {app_token_env: TOKEN}\n  routes:\n"+routes)
		input.File = &file
		if _, err := config.Load(input); err == nil {
			t.Errorf("accepted invalid route list %q", routes)
		}
	}
	file := filepath.Join(input.CWD, "unconfigured-source.yaml")
	writeFile(t, file, "listen:\n  routes:\n    - {name: route, source: slack, type: event, run: [/bin/true]}\n")
	input.File = &file
	if _, err := config.Load(input); err == nil {
		t.Fatal("accepted a route for an unconfigured Slack source")
	}
}

// Native action configuration is a strict exclusive union, selectors cannot be
// templated, and action details remain hidden from effective-config JSON.
func TestNativeRouteConfig(t *testing.T) {
	input := fixture(t)
	file := filepath.Join(input.CWD, "native.yaml")
	writeFile(t, file, `send: {via: hcom, to: reviewer}
notify: {via: apprise}
apprise: {tag: inherited}
listen:
  slack: {app_token_env: TOKEN}
  routes:
    - name: notify-route
      source: slack
      type: channel_created
      notify:
        message: 'Channel ${data.event.channel.id} ($${literal})'
        title: 'Alert ${id}'
        type: warning
        via: apprise
        apprise_tag: urgent
    - name: send-route
      source: slack
      type: channel_created
      send:
        message: 'Inspect channel ${data.event.channel.id}; event ${id}'
        to: reviewer
        intent: inform
`)
	input.File = &file
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Listen.Routes) != 2 || cfg.Listen.Routes[0].Notify == nil || cfg.Listen.Routes[1].Send == nil {
		t.Fatalf("native action union was not parsed: %+v", cfg.Listen.Routes)
	}
	if !cfg.Listen.Routes[0].Notify.TitleSet || cfg.Listen.Routes[0].Notify.AppriseTag != "urgent" || cfg.Listen.Routes[1].Send.Intent != "inform" {
		t.Fatalf("native action options were lost: %+v", cfg.Listen.Routes)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "Channel ${") || strings.Contains(string(encoded), "Inspect channel") || strings.Contains(string(encoded), "urgent") {
		t.Fatalf("inspection exposed action configuration: %s", encoded)
	}

	for _, route := range []string{
		"- {name: no-action, source: slack, type: e}\n",
		"- {name: mixed, source: slack, type: e, run: [/bin/true], notify: {message: hi}}\n",
		"- {name: mixed-native, source: slack, type: e, notify: {message: hi}, send: {message: hi}}\n",
		"- {name: null-action, source: slack, type: e, notify: null}\n",
		"- {name: empty-action, source: slack, type: e, notify: {}}\n",
		"- {name: unknown-action-field, source: slack, type: e, notify: {message: hi, command: [/bin/sh]}}\n",
		"- {name: duplicate-action-field, source: slack, type: e, notify: {message: hi, message: again}}\n",
		"- {name: invalid-template, source: slack, type: e, notify: {message: '${data..id}'}}\n",
		"- {name: unterminated-template, source: slack, type: e, send: {message: '${id'}}\n",
		"- {name: templated-selector, source: slack, type: e, notify: {message: hi, via: '${id}'}}\n",
		"- {name: unsafe-send-selector, source: slack, type: e, send: {message: hi, via: hcom, to: '@reviewer'}}\n",
		"- {name: invalid-send-transport, source: slack, type: e, send: {message: hi, via: remote, to: reviewer}}\n",
		"- {name: request-intent, source: slack, type: e, send: {message: hi, intent: request}}\n",
	} {
		writeFile(t, file, "listen:\n  slack: {app_token_env: TOKEN}\n  routes:\n"+route)
		if _, err := config.Load(input); err == nil {
			t.Errorf("accepted invalid native route %q", route)
		}
	}
}

func TestOutboundSendFlagsDoNotChangeListenerSelectorValidation(t *testing.T) {
	input := fixture(t)
	file := filepath.Join(input.CWD, "native-send.yaml")
	input.File = &file
	input.Overrides = map[string]config.Value{
		"send.via": {Text: "herdr", Source: "flag:--send-via"},
		"send.to":  {Text: "w1:p2", Source: "flag:--to"},
	}
	writeFile(t, file, `send: {via: hcom, to: reviewer}
listen:
  slack: {app_token_env: TOKEN}
  routes:
    - name: inherited-send
      source: slack
      type: event
      send: {message: hi}
`)
	cfg, err := config.Load(input)
	if err != nil || cfg.Send.Via != "herdr" || cfg.Send.To != "w1:p2" {
		t.Fatalf("outbound overrides or inherited listener selectors: %+v %v", cfg, err)
	}

	writeFile(t, file, `send: {via: hcom, to: w1:p2}
listen:
  slack: {app_token_env: TOKEN}
  routes:
    - name: inherited-send
      source: slack
      type: event
      send: {message: hi}
`)
	if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "invalid for its transport") {
		t.Fatalf("outbound override hid invalid inherited listener selectors: %v", err)
	}
}

func TestTemplateGrammar(t *testing.T) {
	parts, err := config.ParseTemplate("literal $${id} ${data.event.channel.id}")
	if err != nil || !reflect.DeepEqual(parts, []config.TemplatePart{{Text: "literal ${id} "}, {Path: "data.event.channel.id"}}) {
		t.Fatalf("template parts: %+v %v", parts, err)
	}
	for _, malformed := range []string{"${}", "${data..id}", "${id", "${a[0]}"} {
		if _, err := config.ParseTemplate(malformed); err == nil {
			t.Errorf("accepted malformed template %q", malformed)
		}
	}
}

// A higher-priority destination must win without rebasing inherited provider
// paths against the checkout or discarding other fields from the same section.
func TestLoadLayersAndPaths(t *testing.T) {
	input := fixture(t)
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, user, "timeout: 3s\nnotify: {via: desktop}\napprise:\n  config: ../apprise/notifications.yaml\n  tag: personal\n  format: markdown\n")
	shared := filepath.Join(input.CWD, "tell.yaml")
	writeFile(t, shared, "apprise: {tag: team}\n")
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != "3s" || cfg.Notify.Via != "desktop" || cfg.Apprise.Tag != "team" || cfg.Apprise.Format != "markdown" {
		t.Fatalf("incorrect precedence: %+v", cfg)
	}
	if cfg.MCP.AgentTargets || cfg.Values()["mcp.agent_targets"] != "false" || cfg.Origins["mcp.agent_targets"] != "default" {
		t.Fatalf("MCP target override is not default-deny: %+v", cfg)
	}
	wantPath := filepath.Join(input.UserConfigDir, "apprise", "notifications.yaml")
	if cfg.Apprise.Config != wantPath || cfg.Origins["apprise.config"] != user || cfg.Origins["apprise.tag"] != shared {
		t.Fatalf("lost path provenance: %+v", cfg)
	}
	if strings.Join(cfg.Sources, "|") != strings.Join([]string{user, shared}, "|") {
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

// A PR-controlled shared file must never redirect delivery, even if a later
// trusted source overrides it. The same fields remain usable in supported personal sources.
func TestPersonalRoutingSources(t *testing.T) {
	for _, tc := range []struct{ key, data, value string }{
		{"notify.via", "notify:\n  via: apprise\n", "apprise"},
		{"apprise.config", "apprise:\n  config: provider.yaml\n", "provider.yaml"},
		{"herdr.session", "herdr:\n  session: personal\n", "personal"},
		{"send.to", "send:\n  to: reviewer\n", "reviewer"},
		{"send.via", "send:\n  via: hcom\n", "hcom"},
		{"send.from", "send:\n  from: nightly\n", "nightly"},
		{"mcp.agent_targets", "mcp:\n  agent_targets: true\n", "true"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			input := fixture(t)
			shared := filepath.Join(input.CWD, "tell.yaml")
			writeFile(t, shared, tc.data)
			input.Overrides = map[string]config.Value{tc.key: {Text: tc.value, Source: "flag"}}
			_, err := config.Load(input)
			if err == nil {
				t.Fatal("accepted shared routing")
			}
			for _, want := range []string{shared + ":2:", tc.key, "user config", ".git/tell.yaml", "--config", "environment", "flags"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("missing %q in diagnostic: %v", want, err)
				}
			}
			writeFile(t, shared, "version: 1\n")
			input.Overrides = nil
			for _, layer := range []string{"user", "explicit"} {
				t.Run(layer, func(t *testing.T) {
					personal := input
					path := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
					if layer == "explicit" {
						path = filepath.Join(input.CWD, "personal.yaml")
						personal.File = &path
					}
					writeFile(t, path, tc.data)
					cfg, err := config.Load(personal)
					if err != nil {
						t.Fatal(err)
					}
					want := tc.value
					if tc.key == "apprise.config" {
						want = filepath.Join(filepath.Dir(path), want)
					}
					if cfg.Values()[tc.key] != want || cfg.Origins[tc.key] != path {
						t.Fatalf("lost personal routing: %+v", cfg)
					}
				})
			}
		})
	}
}

// Shared repositories and invocation overrides cannot weaken personal policy.
func TestSecretPolicySources(t *testing.T) {
	input := fixture(t)
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, user, "secrets: {policy: reject}\n")
	shared := filepath.Join(input.CWD, "tell.yaml")
	writeFile(t, shared, "secrets: {policy: redact}\n")
	if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "personal policy") {
		t.Fatalf("accepted shared secret policy: %v", err)
	}
	writeFile(t, shared, "version: 1\n")
	for _, layer := range []string{"user", "explicit"} {
		personal := input
		path := user
		if layer == "explicit" {
			path = filepath.Join(input.CWD, "personal.yaml")
			personal.File = &path
		}
		writeFile(t, path, "secrets: {policy: reject}\n")
		cfg, err := config.Load(personal)
		if err != nil || cfg.Secrets.Policy != "reject" || cfg.Origins["secrets.policy"] != path {
			t.Fatalf("personal policy %s: %+v %v", layer, cfg, err)
		}
		writeFile(t, path, "secrets: {policy: disabled}\n")
		if _, err := config.Load(personal); err == nil {
			t.Fatal("accepted disabled scanning")
		}
		writeFile(t, path, "version: 1\n")
	}
	input.Overrides = map[string]config.Value{"secrets.policy": {Text: "redact", Source: "flag"}}
	if _, err := config.Load(input); err == nil {
		t.Fatal("accepted non-file policy override")
	}
}

// Shared config cannot introduce executable policy; explicitly selected
// personal files can, and command argv is preserved as a complete list.
func TestCommandConfig(t *testing.T) {
	input := fixture(t)
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	shared := filepath.Join(input.CWD, "tell.yaml")
	explicit := filepath.Join(input.CWD, "personal.yaml")
	writeFile(t, user, "notify: {command: [first, inherited]}\n")
	writeFile(t, explicit, "notify: {via: command, command: [second, '', 'two words']}\n")
	input.File = &explicit
	cfg, err := config.Load(input)
	want := []string{"second", "", "two words"}
	if err != nil || !reflect.DeepEqual(cfg.Notify.Command, want) || cfg.Origins["notify.command"] != explicit {
		t.Fatalf("explicit argv: %+v %v", cfg, err)
	}
	input.File = nil
	writeFile(t, shared, "notify: {command: [untrusted]}\n")
	if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "personal policy") {
		t.Fatalf("accepted shared executable policy: %v", err)
	}
	input.File = &shared
	cfg, err = config.Load(input)
	if err != nil || !reflect.DeepEqual(cfg.Notify.Command, []string{"untrusted"}) {
		t.Fatalf("explicit shared-named config: %+v %v", cfg, err)
	}
	input.Overrides = map[string]config.Value{"notify.command": {Text: "override", Source: "flag"}}
	if _, err := config.Load(input); err == nil {
		t.Fatal("accepted string argv override")
	}
}

// Repository-personal config lives where checkouts cannot write it and must be
// found from every linked worktree, ranked above shared config and below
// invocation overrides. A broken .git pointer must not silently drop routing.
func TestRepositoryPersonalConfig(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(base, "repo")
	user := filepath.Join(base, "user", "tell", "config.yaml")
	shared := filepath.Join(main, "tell.yaml")
	local := filepath.Join(main, ".git", "tell.yaml")
	writeFile(t, user, "notify: {via: desktop}\nherdr: {session: user}\napprise: {tag: user}\n")
	writeFile(t, shared, "apprise: {tag: shared}\n")
	writeFile(t, local, "notify: {via: command, command: [notifier]}\nherdr: {session: repo}\nsend: {via: hcom, to: reviewer}\n")
	worktree := filepath.Join(main, ".worktrees", "feature")
	writeFile(t, filepath.Join(worktree, ".git"), "gitdir: ../../.git/worktrees/feature\n")
	writeFile(t, filepath.Join(main, ".git", "worktrees", "feature", "commondir"), "../..\n")
	for _, cwd := range []string{main, worktree} {
		input := config.Input{CWD: cwd, UserConfigDir: filepath.Join(base, "user"), Overrides: map[string]config.Value{
			"send.to": {Text: "flagged", Source: "flag:--to"},
		}}
		cfg, err := config.Load(input)
		if err != nil {
			t.Fatal(err)
		}
		wantSources := []string{user, local}
		if cwd == main {
			wantSources = []string{user, shared, local}
		}
		if !reflect.DeepEqual(cfg.Sources, wantSources) || cfg.Notify.Via != "command" || cfg.Origins["notify.command"] != local ||
			cfg.Herdr.Session != "repo" || cfg.Send.To != "flagged" || cfg.Send.Via != "hcom" {
			t.Fatalf("repository layer from %s: %+v", cwd, cfg)
		}
		if cwd == main && cfg.Apprise.Tag != "shared" {
			t.Fatalf("lost shared setting: %+v", cfg)
		}
		explicit := filepath.Join(base, "explicit.yaml")
		writeFile(t, explicit, "version: 1\n")
		input.File = &explicit
		if cfg, err := config.Load(input); err != nil || cfg.Notify.Via != "stderr" || len(cfg.Sources) != 1 {
			t.Fatalf("--config inherited repository layer: %+v %v", cfg, err)
		}
	}
	writeFile(t, filepath.Join(worktree, ".git"), "../../.git\n")
	if _, err := config.Load(config.Input{CWD: worktree}); err == nil {
		t.Fatal("accepted malformed .git file")
	}
}

// --config must not load broken automatic sources or accidentally inherit a
// personal destination in CI. Overrides still apply to the selected file.
func TestExplicitConfigIsolatesAutomaticFiles(t *testing.T) {
	input := fixture(t)
	writeFile(t, filepath.Join(input.UserConfigDir, "tell", "config.yaml"), "invalid: true\n")
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

// Commenting out every setting leaves an empty layer rather than breaking delivery.
func TestCommentOnlyFileIsEmptyLayer(t *testing.T) {
	input := fixture(t)
	shared := filepath.Join(input.CWD, "tell.yaml")
	writeFile(t, shared, "# version: 1\n# notify: {via: apprise}\n\n")
	cfg, err := config.Load(input)
	if err != nil || cfg.Notify.Via != "stderr" || cfg.Sources[len(cfg.Sources)-1] != shared {
		t.Fatalf("comment-only layer: %+v %v", cfg, err)
	}
}

// Typos, duplicate fields and YAML indirection can silently redirect messages.
// Validate each source even if a later layer supplies a valid replacement.
func TestRejectAmbiguousOrInvalidConfiguration(t *testing.T) {
	cases := []struct{ name, data string }{
		{"command string", "notify: {command: secret-sentinel}\n"},
		{"command empty", "notify: {command: []}\n"},
		{"command empty executable", "notify: {command: [' ', arg]}\n"},
		{"command non-string", "notify: {command: [exe, 3]}\n"},
		{"command nested", "notify: {command: [exe, [arg]]}\n"},
		{"command anchor", "notify: {command: [exe, &a secret-sentinel]}\n"},
		{"command NUL", "notify: {command: [exe, \"secret-sentinel\\0\"]}\n"},
		{"unknown nested key", "apprise: {tags: ops}\n"},
		{"duplicate nested key", "apprise: {tag: first, tag: second}\n"},
		{"dotted alias for nested key", "notify.via: apprise\nnotify: {via: herdr}\n"},
		{"alias", "notify: &route {via: apprise}\napprise: *route\n"},
		{"merge", "notify: {<<: {via: herdr}}\n"},
		{"null", "apprise: {tag: null}\n"},
		{"wrong type", "apprise: {tag: [ops]}\n"},
		{"MCP setting is not a YAML boolean", "mcp: {agent_targets: 'true'}\n"},
		{"multiple documents", "timeout: 1s\n---\ntimeout: 2s\n"},
		{"unsupported version", "version: 2\n"},
		{"unbounded timeout", "timeout: 0s\n"},
		{"credential URL", "apprise: {config: 'https://user:secret-sentinel@example.com/config'}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := fixture(t)
			user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
			writeFile(t, user, tc.data)
			writeFile(t, filepath.Join(input.CWD, "tell.yaml"), "timeout: 2s\napprise: {tag: valid}\n")
			_, err := config.Load(input)
			if err == nil || !strings.Contains(err.Error(), user) {
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
	if cfg.Project != repo || cfg.Notify.Via != "stderr" {
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

// A dangling shared-config symlink is broken configuration, not permission to
// silently fall back. Non-files must not be consumed.
func TestBrokenConfigPathsFail(t *testing.T) {
	input := fixture(t)
	shared := filepath.Join(input.CWD, "tell.yaml")
	if err := os.Symlink(filepath.Join(input.CWD, "missing.yaml"), shared); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(input); err == nil {
		t.Fatal("ignored dangling shared config")
	}
	directory := input.CWD
	input.File = &directory
	if _, err := config.Load(input); err == nil {
		t.Fatal("accepted a directory as config")
	}
}

// Init creates only shared tell.yaml and preserves it on later calls.
func TestInitPreservesExistingFiles(t *testing.T) {
	input := fixture(t)
	path, created, err := config.Init(input)
	if err != nil || !created {
		t.Fatalf("init: %t %v", created, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Compare against a normal shared file so a restrictive caller umask is
	// honored without changing the process-wide umask in the test.
	reference := filepath.Join(input.CWD, "reference.yaml")
	if err := os.WriteFile(reference, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	referenceInfo, err := os.Stat(reference)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != referenceInfo.Mode().Perm() {
		t.Fatalf("shared mode: %o, want %o", info.Mode().Perm(), referenceInfo.Mode().Perm())
	}
	writeFile(t, path, "sentinel\n")
	_, created, err = config.Init(input)
	if err != nil || created {
		t.Fatalf("repeat init: %t %v", created, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "sentinel\n" {
		t.Fatal("init changed the existing file")
	}
	symlinkInput := fixture(t)
	shared := filepath.Join(symlinkInput.CWD, "tell.yaml")
	if err := os.Symlink(path, shared); err != nil {
		t.Fatal(err)
	}
	if _, _, err := config.Init(symlinkInput); err == nil {
		t.Fatal("init accepted a symlink")
	}
}
