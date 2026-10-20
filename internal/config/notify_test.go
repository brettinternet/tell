package config_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brettinternet/tell/internal/config"
)

// Ordered notify steps replace as a unit, step settings beat file defaults,
// and invocation selectors then override every step of that provider.
func TestNotifyCascadeConfigAndPrecedence(t *testing.T) {
	input := fixture(t)
	user := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, user, `notify:
  via:
    - apprise
    - apprise: {tag: phone, config: first.yaml, format: text}
    - herdr: {session: step-session}
  fallback_on: [not_sent, failed]
  fallback_notice: title
apprise: {config: default.yaml, tag: personal, format: markdown}
herdr: {session: default-session}
`)
	shared := filepath.Join(input.CWD, "tell.yaml")
	writeFile(t, shared, "apprise: {tag: shared}\n")
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := config.EffectiveNotifySteps(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := []config.NotifyStep{
		{Via: "apprise", Config: filepath.Join(input.UserConfigDir, "tell", "default.yaml"), Tag: "shared", Format: "markdown"},
		{Via: "apprise", Config: filepath.Join(input.UserConfigDir, "tell", "first.yaml"), Tag: "phone", Format: "text"},
		{Via: "herdr", Session: "step-session"},
	}
	if !reflect.DeepEqual(steps, want) || cfg.Notify.Via != "apprise" || len(cfg.Notify.Steps) != 3 || !reflect.DeepEqual(cfg.Notify.FallbackOn, []string{"not_sent", "failed"}) || cfg.Notify.FallbackNotice != "title" {
		t.Fatalf("resolved cascade: cfg=%+v steps=%+v", cfg.Notify, steps)
	}
	input.Overrides = map[string]config.Value{
		"apprise.tag":    {Text: "override", Source: "flag:--apprise-tag"},
		"apprise.format": {Text: "html", Source: "env:TELL_APPRISE_FORMAT"},
		"herdr.session":  {Text: "override-session", Source: "mcp:notify"},
	}
	cfg, err = config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	steps, err = config.EffectiveNotifySteps(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if steps[0].Tag != "override" || steps[1].Tag != "override" || steps[0].Format != "html" || steps[1].Format != "html" || steps[2].Session != "override-session" {
		t.Fatalf("one-shot overrides did not apply to each matching step: %+v", steps)
	}

	// A higher file layer replaces the ordered list wholesale rather than merging it.
	repository := filepath.Join(input.CWD, "..", "worktree-metadata", "tell.yaml")
	writeFile(t, repository, "notify: {via: [stderr]}\n")
	input.Overrides = nil
	cfg, err = config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Notify.Steps) != 1 || cfg.Notify.Steps[0].Via != "stderr" || cfg.Origins["notify.via"] != repository {
		t.Fatalf("higher list layer did not replace lower list: %+v", cfg.Notify)
	}

	// A via override deliberately converts the cascade to one selected route.
	input.Overrides = map[string]config.Value{"notify.via": {Text: "desktop", Source: "flag:--via"}}
	cfg, err = config.Load(input)
	if err != nil || cfg.Notify.Via != "desktop" || len(cfg.Notify.Steps) != 0 {
		t.Fatalf("via override did not disable cascade: %+v %v", cfg.Notify, err)
	}
}

// Invalid or ambiguous cascades must fail config loading before a provider can run.
func TestNotifyCascadeStrictValidation(t *testing.T) {
	for _, data := range []string{
		"notify: {via: []}\n",
		"notify:\n  via: [apprise, {apprise: {tag: phone}, herdr: {session: x}}]\n",
		"notify: {via: [{apprise: {unknown: value}}]}\n",
		"notify: {via: [{stderr: {tag: value}}]}\n",
		"notify: {via: [{apprise: {urls_env: 'BAD-NAME'}}]}\n",
		"notify: {via: [{apprise: {urls_env: URLS, config: config.yaml}}]}\n",
		"notify: {via: [{apprise: {urls_env: URLS, tag: phone}}]}\n",
		// Inherited defaults must not silently disappear when choosing URL-based targets.
		"notify: {via: [{apprise: {urls_env: URLS}}]}\napprise: {tag: phone}\n",
		"notify: {via: [{apprise: {urls_env: URLS}}]}\napprise: {config: config.yaml}\n",
		"notify: {via: [apprise], fallback_on: [uncertain]}\n",
		"notify: {via: [apprise], fallback_on: [config]}\n",
		"notify: {via: [apprise], fallback_on: [usage]}\n",
		"notify: {via: [apprise], fallback_notice: invalid}\n",
		"notify: {via: [command, stderr]}\n",
		"notify: {via: apprise, fallback_on: [not_sent]}\n",
		"notify: {via: apprise, fallback_notice: title}\n",
	} {
		input := fixture(t)
		file := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
		writeFile(t, file, data)
		if _, err := config.Load(input); err == nil {
			t.Errorf("accepted invalid notify config %q", data)
		}
	}
	input := fixture(t)
	writeFile(t, filepath.Join(input.CWD, "tell.yaml"), "notify: {via: [stderr, desktop]}\n")
	if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "personal routing") {
		t.Fatalf("shared cascade was accepted: %v", err)
	}

	input = fixture(t)
	file := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, file, "notify: {via: [{apprise: {urls_env: PROVIDER_URLS}}]}\n")
	input.Overrides = map[string]config.Value{"apprise.tag": {Text: "one-shot", Source: "env:TELL_APPRISE_TAG"}}
	if _, err := config.Load(input); err == nil || !strings.Contains(err.Error(), "urls_env") {
		t.Fatalf("one-shot tag did not conflict with urls_env: %v", err)
	}

	input = fixture(t)
	file = filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, file, "notify: {via: [command, stderr], fallback_on: [failed], fallback_notice: title}\n")
	input.Overrides = map[string]config.Value{"notify.via": {Text: "stderr", Source: "flag:--via"}}
	cfg, err := config.Load(input)
	if err != nil || cfg.Notify.Via != "stderr" || len(cfg.Notify.Steps) != 0 || len(cfg.Notify.FallbackOn) != 0 || cfg.Notify.FallbackNotice != "" {
		t.Fatalf("via override did not disable an unselected command step: %+v %v", cfg.Notify, err)
	}
}

// Config inspection carries only URLs environment-variable names, never their values.
func TestNotifyCascadeInspectionDoesNotReadProviderValues(t *testing.T) {
	input := fixture(t)
	file := filepath.Join(input.UserConfigDir, "tell", "config.yaml")
	writeFile(t, file, "notify: {via: [{apprise: {urls_env: PRIVATE_URLS}}]}\n")
	t.Setenv("PRIVATE_URLS", "https://user:private-url-secret@example.invalid")
	cfg, err := config.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "private-url-secret") || !strings.Contains(string(encoded), "PRIVATE_URLS") || strings.Contains(cfg.Values()["notify.via"], "private-url-secret") {
		t.Fatalf("cascade inspection exposed environment contents: %s", encoded)
	}
}
