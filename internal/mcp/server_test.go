package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/brettinternet/tell/internal/config"
	tellmcp "github.com/brettinternet/tell/internal/mcp"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServeStdioDrainsResponsesAfterEOF(t *testing.T) {
	root := t.TempDir()
	var serverStderr bytes.Buffer
	server := tellmcp.NewServer(config.Input{CWD: root}, "test", &serverStderr)
	lines := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"stdio-test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"notify","arguments":{"message":"PRIVATE-BODY-SENTINEL"},"_meta":[]}}`,
	}
	var output bytes.Buffer
	if err := tellmcp.ServeStdio(context.Background(), server, strings.NewReader(strings.Join(lines, "\n")+"\n"), &output); err != nil {
		t.Fatalf("ServeStdio: %v", err)
	}
	responses := map[float64]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		var response map[string]any
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatalf("invalid stdio response %q: %v", line, err)
		}
		if id, ok := response["id"].(float64); ok {
			responses[id] = response
		}
	}
	if len(responses) != 3 {
		t.Fatalf("got %d protocol responses after EOF, want initialize, tools/list, and malformed tools/call: %s", len(responses), output.String())
	}
	list := responses[2]["result"].(map[string]any)["tools"].([]any)
	if len(list) != 3 {
		t.Fatalf("tools/list returned %d tools, want three: %s", len(list), output.String())
	}
	failure, ok := responses[3]["error"].(map[string]any)
	if !ok || responses[3]["id"] != float64(3) || failure["code"] != float64(-32602) || failure["message"] != "request could not be processed" {
		t.Fatalf("malformed tools/call response was not safely preserved: %+v", responses[3])
	}
	if _, ok := failure["data"]; ok {
		t.Fatalf("malformed tools/call error retained data: %+v", failure)
	}
	if strings.Contains(output.String(), "PRIVATE-BODY-SENTINEL") || strings.Contains(serverStderr.String(), "PRIVATE-BODY-SENTINEL") {
		t.Fatal("malformed tools/call leaked its message body to an output stream")
	}
	if serverStderr.String() != "" {
		t.Fatalf("malformed tools/call triggered delivery on stderr: %q", serverStderr.String())
	}
}

// Public MCP calls catch hidden-argument routing bypasses, stale per-call
// configuration, duplicate delivery, and message/provider-output disclosure.
func TestMCPToolsAndRoutingBoundary(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("HOME", root)
	userConfigDir := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", userConfigDir)
	for _, name := range []string{"TELL_PROJECT", "TELL_CONFIG", "TELL_MCP_AGENT_TARGETS", "TELL_NOTIFY_VIA", "TELL_SEND_VIA", "TELL_SEND_TO", "TELL_SEND_FROM", "TELL_HERDR_SESSION", "TELL_APPRISE_TAG", "TELL_TIMEOUT"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	providerDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(providerDir, 0o700); err != nil {
		t.Fatal(err)
	}
	appriseLog := filepath.Join(root, "apprise.args")
	hcomLog := filepath.Join(root, "hcom.args")
	herdrLog := filepath.Join(root, "herdr.args")
	writeExecutable(t, filepath.Join(providerDir, "apprise"), `#!/bin/sh
printf '%s\000' "$@" > "$MCP_APPRISE_LOG"
if [ "$MCP_APPRISE_WAIT" = yes ]; then
  exec /bin/sleep 5
fi
printf 'raw-provider-output body-leak-sentinel\n'
printf 'raw-provider-output body-leak-sentinel\n' >&2
exit "${MCP_APPRISE_EXIT:-0}"
`)
	writeExecutable(t, filepath.Join(providerDir, "hcom"), `#!/bin/sh
if [ "$1" = list ]; then
  printf '[{"name":"reviewer","status":"active"}]'
  exit 0
fi
printf '%s\000' "$@" >> "$MCP_HCOM_LOG"
printf 'raw-provider-output body-leak-sentinel\n'
printf 'raw-provider-output body-leak-sentinel\n' >&2
exit 0
`)
	writeExecutable(t, filepath.Join(providerDir, "herdr"), `#!/bin/sh
if [ "$3" = agent ] && [ "$4" = get ]; then
  printf '{"result":{"agent":{"agent_status":"idle","pane_id":"w1:p2"}}}'
  exit 0
fi
if [ "$3" = agent ] && [ "$4" = prompt ]; then
  printf '%s\000' "$@" >> "$MCP_HERDR_LOG"
  printf '{"result":{"receipt":"raw-provider-output body-leak-sentinel"}}'
  printf 'raw-provider-output body-leak-sentinel\n' >&2
  exit 0
fi
exit 1
`)
	t.Setenv("PATH", providerDir)
	t.Setenv("MCP_APPRISE_LOG", appriseLog)
	t.Setenv("MCP_HCOM_LOG", hcomLog)
	t.Setenv("MCP_HERDR_LOG", herdrLog)
	t.Setenv("MCP_APPRISE_EXIT", "0")
	t.Setenv("MCP_APPRISE_WAIT", "")
	configPath := filepath.Join(userConfigDir, "tell", "config.yaml")
	writeConfig(t, configPath, "send:\n  via: hcom\n  to: reviewer\n  from: automation\n")
	input := config.Input{CWD: root, UserConfigDir: userConfigDir}
	var serverStderr strings.Builder
	server := tellmcp.NewServer(input, "test", &serverStderr)
	client, closeClient := connect(t, server)

	listed, err := client.ListTools(context.Background(), &protocol.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(listed.Tools))
	tools := make(map[string]*protocol.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
		tools[tool.Name] = tool
	}
	if !reflect.DeepEqual(names, []string{"doctor", "notify", "send"}) {
		t.Fatalf("tools/list = %v, want exactly doctor, notify, send", names)
	}
	for name, forbidden := range map[string][]string{
		"notify": {"via", "herdr_session", "apprise_tag", "command", "apprise_config", "timeout", "secrets_policy"},
		"send":   {"via", "to", "herdr_session", "from", "timeout", "secrets_policy"},
	} {
		properties := toolProperties(t, tools[name])
		message, ok := properties["message"].(map[string]any)
		if !ok || message["type"] != "string" {
			t.Errorf("%s message schema is not an unconstrained string: %+v", name, properties["message"])
		}
		if _, constrained := message["enum"]; constrained {
			t.Errorf("%s message schema unexpectedly restricts message text", name)
		}
		for _, field := range forbidden {
			if _, ok := properties[field]; ok {
				t.Errorf("default %s schema exposed %q", name, field)
			}
		}
	}
	if tools["doctor"].Annotations == nil || !tools["doctor"].Annotations.ReadOnlyHint {
		t.Fatal("doctor must carry readOnlyHint")
	}
	doctorSchema := tools["doctor"].InputSchema.(map[string]any)
	_, requiredDoctorArguments := doctorSchema["required"]
	if len(toolProperties(t, tools["doctor"])) != 0 || requiredDoctorArguments {
		t.Fatalf("doctor schema must be an empty optional object: %+v", doctorSchema)
	}
	doctor := callTool(t, client, "doctor", map[string]any{})
	if doctor.IsError || structured(t, doctor)["ready"] != true {
		t.Fatalf("doctor result: %+v", structured(t, doctor))
	}

	deniedNotify := callTool(t, client, "notify", map[string]any{"message": "body-leak-sentinel", "via": "apprise"})
	assertFailure(t, deniedNotify, "usage", "agent_targets_disabled")
	if _, err := os.Stat(appriseLog); !os.IsNotExist(err) {
		t.Fatalf("default-deny notify started apprise: %v", err)
	}
	deniedSend := callTool(t, client, "send", map[string]any{"message": "body-leak-sentinel", "to": "attacker"})
	assertFailure(t, deniedSend, "usage", "agent_targets_disabled")
	if _, err := os.Stat(hcomLog); !os.IsNotExist(err) {
		t.Fatalf("default-deny send dispatched hcom: %v", err)
	}

	stderrNotify := callTool(t, client, "notify", map[string]any{"message": "stderr-body-sentinel"})
	assertDelivery(t, stderrNotify, "stderr", "accepted", "", "")
	if serverStderr.String() != "stderr-body-sentinel\n" {
		t.Fatalf("stderr notify went to the wrong stream: %q", serverStderr.String())
	}
	assertNoMessageOrProviderOutput(t, stderrNotify, "stderr-body-sentinel")

	defaultSend := callTool(t, client, "send", map[string]any{"message": "hcom-body-sentinel"})
	assertDelivery(t, defaultSend, "hcom", "accepted", "reviewer", "inform")
	assertNoMessageOrProviderOutput(t, defaultSend, "hcom-body-sentinel")
	if data, err := os.ReadFile(hcomLog); err != nil || !strings.Contains(string(data), "@reviewer") {
		t.Fatalf("configured hcom recipient was not used: %q %v", data, err)
	}

	// The same live process must use the current layered config on the next call.
	writeConfig(t, configPath, "notify:\n  via: apprise\nsend:\n  via: hcom\n  to: reviewer\n  from: automation\n")
	updated := callTool(t, client, "notify", map[string]any{"message": "configured-apprise-body"})
	assertDelivery(t, updated, "apprise", "accepted", "", "")
	assertNoMessageOrProviderOutput(t, updated, "configured-apprise-body")
	if data, err := os.ReadFile(appriseLog); err != nil || !strings.Contains(string(data), "configured-apprise-body") {
		t.Fatalf("per-call configuration edit was ignored: %q %v", data, err)
	}

	// Bad config becomes a body-free tool error and does not terminate serving.
	writeConfig(t, configPath, "notify: {via: [invalid]}\n")
	badConfig := callTool(t, client, "notify", map[string]any{"message": "must-not-send"})
	assertFailure(t, badConfig, "config", "invalid_config")
	writeConfig(t, configPath, "notify: {via: stderr}\n")
	recovered := callTool(t, client, "notify", map[string]any{"message": "after-config-fix"})
	assertDelivery(t, recovered, "stderr", "accepted", "", "")

	defer closeClient()
	// A live permission toggle must update discovery as well as enforcement;
	// otherwise clients cannot discover allowed selectors or retain revoked ones.
	for _, enabled := range []bool{true, false} {
		text := "mcp: {agent_targets: false}\n"
		if enabled {
			text = "mcp: {agent_targets: true}\n"
		}
		writeConfig(t, configPath, text)
		listing, err := client.ListTools(context.Background(), &protocol.ListToolsParams{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tool := range listing.Tools {
			if tool.Name == "doctor" {
				continue
			}
			_, hasSelector := toolProperties(t, tool)["via"]
			if hasSelector != enabled {
				t.Fatalf("live %s schema selector = %v, want %v", tool.Name, hasSelector, enabled)
			}
		}
		result := callTool(t, client, "notify", map[string]any{"message": "toggle", "via": "stderr"})
		if enabled {
			assertDelivery(t, result, "stderr", "accepted", "", "")
		} else {
			assertFailure(t, result, "usage", "agent_targets_disabled")
		}
	}
	configText := "mcp:\n  agent_targets: true\nnotify:\n  via: stderr\nsend:\n  via: hcom\n  to: reviewer\n  from: automation\n"
	writeConfig(t, configPath, configText)
	serverStderr.Reset()
	listed, err = client.ListTools(context.Background(), &protocol.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	tools = make(map[string]*protocol.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		tools[tool.Name] = tool
	}
	for name, required := range map[string][]string{
		"notify": {"via", "herdr_session", "apprise_tag"},
		"send":   {"via", "to", "herdr_session"},
	} {
		properties := toolProperties(t, tools[name])
		for _, field := range required {
			if _, ok := properties[field]; !ok {
				t.Errorf("opt-in %s schema omitted %q", name, field)
			}
		}
	}

	overriddenNotify := callTool(t, client, "notify", map[string]any{"message": "override-notify-body", "via": "apprise", "apprise_tag": "mcp-selected"})
	assertDelivery(t, overriddenNotify, "apprise", "accepted", "", "")
	assertNoMessageOrProviderOutput(t, overriddenNotify, "override-notify-body")
	if data, err := os.ReadFile(appriseLog); err != nil || !strings.Contains(string(data), "mcp-selected") {
		t.Fatalf("opt-in Apprise tag override missing: %q %v", data, err)
	}

	t.Setenv("MCP_APPRISE_EXIT", "3")
	noMatch := callTool(t, client, "notify", map[string]any{"message": "not-sent-body", "via": "apprise"})
	assertFailure(t, noMatch, "not_sent", "no_match")
	assertNoMessageOrProviderOutput(t, noMatch, "not-sent-body")
	t.Setenv("MCP_APPRISE_EXIT", "0")
	writeConfig(t, configPath, "mcp: {agent_targets: true}\ntimeout: 100ms\nnotify: {via: stderr}\nsend: {via: hcom, to: reviewer, from: automation}\n")
	t.Setenv("MCP_APPRISE_WAIT", "yes")
	uncertain := callTool(t, client, "notify", map[string]any{"message": "uncertain-body", "via": "apprise"})
	assertFailure(t, uncertain, "uncertain", "timeout")
	assertNoMessageOrProviderOutput(t, uncertain, "uncertain-body")
	t.Setenv("MCP_APPRISE_WAIT", "")
	writeConfig(t, configPath, configText)

	before, err := os.ReadFile(hcomLog)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := callTool(t, client, "send", map[string]any{"message": "must-not-send", "from": "attacker"})
	assertFailure(t, forbidden, "usage", "invalid_override")
	after, err := os.ReadFile(hcomLog)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("forbidden sender override dispatched: before=%q after=%q err=%v", before, after, err)
	}

	herdrSend := callTool(t, client, "send", map[string]any{"message": "herdr-body-sentinel", "intent": "request", "via": "herdr", "to": "reviewer", "herdr_session": "selected-session"})
	assertDelivery(t, herdrSend, "herdr", "accepted", "reviewer", "request")
	assertNoMessageOrProviderOutput(t, herdrSend, "herdr-body-sentinel")
	if data, err := os.ReadFile(herdrLog); err != nil || !strings.Contains(string(data), "selected-session") {
		t.Fatalf("opt-in Herdr session override missing: %q %v", data, err)
	}
	invalidTarget := callTool(t, client, "send", map[string]any{"message": "must-not-send", "via": "herdr", "to": "@all", "herdr_session": "selected-session"})
	assertFailure(t, invalidTarget, "not_sent", "invalid_target")
}

func writeConfig(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
}

func connect(t *testing.T, server *protocol.Server) (*protocol.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()
	clientTransport, serverTransport := protocol.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSession, err := protocol.NewClient(&protocol.Implementation{Name: "tell-test", Version: "test"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	closeClient := func() {
		once.Do(func() {
			if err := clientSession.Close(); err != nil {
				t.Errorf("close MCP client: %v", err)
			}
			if err := serverSession.Wait(); err != nil {
				t.Errorf("wait for MCP server: %v", err)
			}
		})
	}
	t.Cleanup(closeClient)
	return clientSession, closeClient
}

func callTool(t *testing.T, client *protocol.ClientSession, name string, args map[string]any) *protocol.CallToolResult {
	t.Helper()
	result, err := client.CallTool(context.Background(), &protocol.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

func toolProperties(t *testing.T, tool *protocol.Tool) map[string]any {
	t.Helper()
	schema, ok := tool.InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("tool %q input schema is %T, want object", tool.Name, tool.InputSchema)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("tool %q properties is %T", tool.Name, schema["properties"])
	}
	return properties
}

func structured(t *testing.T, result *protocol.CallToolResult) map[string]any {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode structured content %s: %v", data, err)
	}
	return value
}

func assertDelivery(t *testing.T, result *protocol.CallToolResult, route, outcome, recipient, intent string) {
	t.Helper()
	value := structured(t, result)
	if result.IsError || value["schema_version"] != float64(1) || value["route"] != route || value["outcome"] != outcome {
		t.Fatalf("delivery result: isError=%t structured=%+v", result.IsError, value)
	}
	if _, ok := value["redactions"]; !ok {
		t.Fatalf("delivery result missing redactions: %+v", value)
	}
	if recipient != "" && value["recipient"] != recipient || intent != "" && value["intent"] != intent {
		t.Fatalf("send result missing receipt fields: %+v", value)
	}
}

func assertFailure(t *testing.T, result *protocol.CallToolResult, outcome, reason string) {
	t.Helper()
	value := structured(t, result)
	failure, ok := value["error"].(map[string]any)
	if !result.IsError || value["outcome"] != outcome || !ok || failure["code"] != outcome || failure["reason"] != reason {
		t.Fatalf("failure result: isError=%t structured=%+v", result.IsError, value)
	}
}

func assertNoMessageOrProviderOutput(t *testing.T, result *protocol.CallToolResult, body string) {
	t.Helper()
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{body, "raw-provider-output", "body-leak-sentinel"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("tool result leaked %q: %s", secret, data)
		}
	}
}
