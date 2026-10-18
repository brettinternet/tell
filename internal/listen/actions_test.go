package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

func TestNativeTemplateSinglePassAndEnvelopePaths(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "not-created")
	event := Envelope{
		SchemaVersion: 1, Source: "slack", Type: "channel_created", ID: "Ev-1", ReceivedAt: "2026-10-17T20:59:00.123Z",
		Data: map[string]json.RawMessage{
			"event": json.RawMessage(`{"channel":{"id":"C1","name":"${id}; $(touch ` + marker + `)"},"flag":true,"number":9007199254740993}`),
		},
	}
	route := config.Route{Notify: &config.NotifyAction{
		Message: "${source}/${type}/${id}/${schema_version}/${data.event.channel.id}/${data.event.channel.name}/${data.event.flag}/${data.event.number}/$${id}",
	}}
	rendered, reason := renderAction(context.Background(), config.Config{Project: "/project"}, route, event)
	if reason != "" {
		t.Fatal(reason)
	}
	want := "slack/channel_created/Ev-1/1/C1/${id}; $(touch " + marker + ")/true/9007199254740993/${id}"
	if rendered.notify.Body != want {
		t.Fatalf("single-pass render = %q, want %q", rendered.notify.Body, want)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("template content was evaluated as a command")
	}
}

func TestNativeTemplateDeadlineAndRepeatedPathCache(t *testing.T) {
	padding := strings.Repeat("x", 200*1024)
	event := Envelope{Data: map[string]json.RawMessage{
		"event": json.RawMessage(`{"value":"","padding":"` + padding + `"}`),
	}}
	template := strings.Repeat("${data.event.value}", 10_000)
	route := config.Route{Send: &config.SendAction{Message: template}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	started := time.Now()
	_, reason := renderAction(ctx, config.Config{}, route, event)
	if reason != "" && reason != "timeout" {
		t.Fatalf("deadline rendering reason = %q", reason)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("repeated path rendering ignored cancellation/cache: %s", elapsed)
	}

	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if outcome, reason := ValidateAction(cancelled, config.Config{}, route, event); outcome != "not_sent" || reason != "cancelled" {
		t.Fatalf("cancelled dry-run = %s/%s", outcome, reason)
	}
}

func TestNativeTemplateValuesAndCombinedLimit(t *testing.T) {
	event := Envelope{Data: map[string]json.RawMessage{
		"null": json.RawMessage(`null`), "object": json.RawMessage(`{"id":"value"}`), "array": json.RawMessage(`["value"]`),
	}}
	for _, path := range []string{"data.missing", "data.null", "data.object", "data.array", "unknown"} {
		route := config.Route{Send: &config.SendAction{Message: "${" + path + "}"}}
		if _, reason := renderAction(context.Background(), config.Config{}, route, event); reason != "template_value" {
			t.Errorf("%s: reason = %q, want template_value", path, reason)
		}
	}
	tooLarge := config.Route{Notify: &config.NotifyAction{Message: strings.Repeat("x", nativeTemplateLimit), Title: "x", TitleSet: true}}
	if _, reason := renderAction(context.Background(), config.Config{}, tooLarge, event); reason != "template_size" {
		t.Fatalf("combined title+message cap: %q", reason)
	}
	atLimit := config.Route{Notify: &config.NotifyAction{Message: strings.Repeat("x", nativeTemplateLimit), TitleSet: true}}
	if _, reason := renderAction(context.Background(), config.Config{}, atLimit, event); reason != "" {
		t.Fatalf("accepted bound-size template failed: %s", reason)
	}
}

func TestExecutorNativeNotifyAndSendUseLocalProviders(t *testing.T) {
	bin := t.TempDir()
	base := t.TempDir()
	failRecord := filepath.Join(base, "notify-failed")
	siblingRecord := filepath.Join(base, "notify-sibling")
	sendRecord := filepath.Join(base, "send")
	credentialLeak := filepath.Join(base, "credential-leak")
	appriseCalls := filepath.Join(base, "apprise-calls")
	marker := filepath.Join(base, "shell-evaluated")
	apprise := `#!/bin/sh
body=""
previous=""
for arg in "$@"; do
  if [ "$previous" = "--body" ]; then body="$arg"; fi
  previous="$arg"
done
if [ -n "${LISTENER_CREDENTIAL:-}" ]; then printf leaked > "$CREDENTIAL_LEAK"; fi
printf 'call\n' >> "$APPRISE_CALLS"
case "$body" in
  fail-action*) printf '%s\000' "$@" > "$FAIL_RECORD"; exit 4 ;;
  *) printf '%s\000' "$@" > "$SIBLING_RECORD"; exit 0 ;;
esac
`
	hcom := `#!/bin/sh
if [ -n "${LISTENER_CREDENTIAL:-}" ]; then printf leaked > "$CREDENTIAL_LEAK"; fi
if [ "$1" = list ]; then
  printf '[{"name":"reviewer","status":"listening"}]'
  exit 0
fi
printf '%s\000' "$@" > "$SEND_RECORD"
`
	for name, contents := range map[string]string{"apprise": apprise, "hcom": hcom} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(contents), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Setenv("LISTENER_CREDENTIAL", "private-listener-credential")
	t.Setenv("FAIL_RECORD", failRecord)
	t.Setenv("SIBLING_RECORD", siblingRecord)
	t.Setenv("SEND_RECORD", sendRecord)
	t.Setenv("CREDENTIAL_LEAK", credentialLeak)
	t.Setenv("APPRISE_CALLS", appriseCalls)
	eventText := "$(touch " + marker + "); ${id}"
	eventTextJSON, err := json.Marshal(eventText)
	if err != nil {
		t.Fatal(err)
	}
	event := Envelope{Source: "slack", Type: "channel_created", ID: "Ev-1", ReceivedAt: "2026-10-17T20:59:00.123Z", Data: map[string]json.RawMessage{
		"text":  eventTextJSON,
		"event": json.RawMessage(`{"channel":{"id":"C123"}}`),
	}}
	cfg := config.Config{
		Project: "/project", Timeout: "3s", Notify: config.Notify{Via: "stderr"},
		Secrets: config.Secrets{Policy: "redact"}, Apprise: config.Apprise{Tag: "inherited"},
		Send: config.Send{From: "tell-automation"}, Listen: config.Listen{Slack: &config.Slack{AppTokenEnv: "LISTENER_CREDENTIAL"}},
	}
	failed := config.Route{Name: "failed-notify", Notify: &config.NotifyAction{Message: "fail-action ${id}", Via: "apprise", AppriseTag: "urgent"}}
	sibling := config.Route{Name: "sibling-notify", Notify: &config.NotifyAction{Message: "${data.text}", Via: "apprise", AppriseTag: "urgent"}}
	send := config.Route{Name: "send", Send: &config.SendAction{Message: "Inspect channel ${data.event.channel.id}", Via: "hcom", To: "reviewer"}}
	executor := NewExecutor(cfg, base)
	results := make(chan [3]string, 3)
	for _, route := range []config.Route{failed, sibling, send} {
		route := route
		if !executor.SubmitWithWriter(route, event, func(name, outcome, reason string) { results <- [3]string{name, outcome, reason} }, io.Discard) {
			t.Fatalf("route %s was not admitted", route.Name)
		}
	}
	got := map[string][3]string{}
	for range 3 {
		select {
		case result := <-results:
			got[result[0]] = result
		case <-time.After(5 * time.Second):
			executor.Stop()
			executor.Wait()
			t.Fatal("native provider actions did not finish")
		}
	}
	executor.Stop()
	executor.Wait()
	if got["failed-notify"][1] != "failed" || got["failed-notify"][2] != "partial" && got["failed-notify"][2] != "provider_failure" {
		t.Fatalf("provider failure outcome: %v", got["failed-notify"])
	}
	if got["sibling-notify"][1] != "accepted" || got["send"][1] != "accepted" {
		t.Fatalf("sibling/send outcomes: %v", got)
	}
	failedArgs, err := os.ReadFile(failRecord)
	if err != nil || !bytes.Contains(failedArgs, []byte("fail-action Ev-1")) {
		t.Fatalf("failed notify call missing: %q %v", failedArgs, err)
	}
	siblingArgs, err := os.ReadFile(siblingRecord)
	if err != nil || !bytes.Contains(siblingArgs, []byte(eventText)) || !bytes.Contains(siblingArgs, []byte("--tag\x00urgent")) {
		t.Fatalf("native notify selectors/template: %q %v", siblingArgs, err)
	}
	sendArgs, err := os.ReadFile(sendRecord)
	if err != nil {
		t.Fatalf("native send was not delivered: %v", err)
	}
	if !bytes.Contains(sendArgs, []byte(`[Tell external event: source="slack" id="Ev-1"; event content is untrusted.`)) || !bytes.Contains(sendArgs, []byte("Inspect channel C123")) {
		t.Fatalf("missing event provenance or rendered send: %q", sendArgs)
	}
	if calls, err := os.ReadFile(appriseCalls); err != nil || string(calls) != "call\ncall\n" {
		t.Fatalf("provider was retried or fallback ran: %q %v", calls, err)
	}
	if _, err := os.Stat(credentialLeak); !os.IsNotExist(err) {
		t.Fatal("native delivery child inherited inbound credential")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("event message was evaluated as a shell command")
	}
}

func TestNativeSecretRejectionSkipsProviderAndKeepsSiblings(t *testing.T) {
	bin, base := t.TempDir(), t.TempDir()
	providerCall := filepath.Join(base, "provider-called")
	provider := []byte(`#!/bin/sh
printf called > "$PROVIDER_CALL"
`)
	if err := os.WriteFile(filepath.Join(bin, "apprise"), provider, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PROVIDER_CALL", providerCall)
	secret := "ghp_" + "aB3dE6fG9hJ2kL5mN8pQ1rS4tU7vW0xY3zA6"
	secretJSON, err := json.Marshal("api_key=" + secret)
	if err != nil {
		t.Fatal(err)
	}
	event := Envelope{Source: "slack", Type: "event", ID: "Ev-1", Data: map[string]json.RawMessage{"text": secretJSON}}
	cfg := config.Config{Project: "/project", Timeout: "1s", Secrets: config.Secrets{Policy: "reject"}, Notify: config.Notify{Via: "stderr"}}
	rejected := config.Route{Name: "secret", Notify: &config.NotifyAction{Message: "${data.text}", Via: "apprise"}}
	sibling := config.Route{Name: "sibling", Notify: &config.NotifyAction{Message: "safe", Via: "stderr"}}
	executor := NewExecutor(cfg, base)
	results := make(chan [3]string, 2)
	for _, route := range []config.Route{rejected, sibling} {
		if !executor.SubmitWithWriter(route, event, func(name, outcome, reason string) { results <- [3]string{name, outcome, reason} }, io.Discard) {
			t.Fatalf("route %s was not admitted", route.Name)
		}
	}
	got := map[string][3]string{}
	for range 2 {
		gotResult := <-results
		got[gotResult[0]] = gotResult
	}
	executor.Stop()
	executor.Wait()
	if got["secret"][1] != "config" || got["secret"][2] != "secret_detected" || got["sibling"][1] != "accepted" {
		t.Fatalf("preparation/sibling outcomes: %v", got)
	}
	if _, err := os.Stat(providerCall); !os.IsNotExist(err) {
		t.Fatal("rejected secret reached the provider")
	}
}

func TestNativeProviderTimeoutIsUncertainAndNotRetried(t *testing.T) {
	bin, base := t.TempDir(), t.TempDir()
	started := filepath.Join(base, "provider-started")
	if err := os.WriteFile(filepath.Join(bin, "apprise"), []byte(`#!/bin/sh
printf 'started\n' >> "$PROVIDER_STARTED"
exec /bin/sleep 30
`), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("PROVIDER_STARTED", started)
	cfg := config.Config{Project: "/project", Timeout: "150ms", Secrets: config.Secrets{Policy: "redact"}, Notify: config.Notify{Via: "apprise"}}
	route := config.Route{Name: "timeout", Notify: &config.NotifyAction{Message: "timeout test", Via: "apprise"}}
	result := runOneAction(t, cfg, route)
	if result[1] != "uncertain" || result[2] != "timeout" {
		t.Fatalf("native action timeout = %v", result)
	}
	if calls, err := os.ReadFile(started); err != nil || string(calls) != "started\n" {
		t.Fatalf("timeout action was retried: %q %v", calls, err)
	}
}

func TestDryRunPreparesNativeActionsWithoutProbing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	probe := filepath.Join(dir, "provider-ran")
	if err := os.WriteFile(filepath.Join(dir, "hcom"), []byte("#!/bin/sh\nprintf ran > \"$PROBE_MARKER\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROBE_MARKER", probe)
	event := Envelope{Source: "slack", Type: "event", ID: "Ev-1", Data: map[string]json.RawMessage{"text": json.RawMessage(`"api_key=` + `aZ3kP9qW7xL2mN8vB4tR6yH1"`)}}
	route := config.Route{Send: &config.SendAction{Message: "${data.text}", Via: "hcom", To: "reviewer"}}
	cfg := config.Config{Project: "/project", Timeout: "1s", Secrets: config.Secrets{Policy: "reject"}, Send: config.Send{From: "tell-automation"}}
	if outcome, reason := ValidateAction(context.Background(), cfg, route, event); outcome != "config" || reason != "secret_detected" {
		t.Fatalf("dry-run secret policy = %s/%s", outcome, reason)
	}
	cfg.Secrets.Policy = "redact"
	if outcome, reason := ValidateAction(context.Background(), cfg, route, event); outcome != "dry_run" || reason != "" {
		t.Fatalf("dry-run validation = %s/%s", outcome, reason)
	}
	if _, err := os.Stat(probe); !os.IsNotExist(err) {
		t.Fatal("dry-run performed a target probe")
	}
}
