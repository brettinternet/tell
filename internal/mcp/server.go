// Package mcp exposes Tell delivery operations as MCP tools.
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/brettinternet/tell/internal/config"
	"github.com/brettinternet/tell/internal/delivery"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer builds an MCP server over the same per-invocation configuration and
// delivery operations as the CLI. stderr is reserved for the stderr notify route.
func NewServer(input config.Input, version string, stderr io.Writer) *protocol.Server {
	if version == "" {
		version = "dev"
	}
	server := protocol.NewServer(&protocol.Implementation{Name: "tell", Version: version}, nil)
	cfg, err := config.Load(input)
	allowTargets := err == nil && cfg.MCP.AgentTargets

	addTool(server, "notify", "Deliver one notification using Tell's configured route or fallback cascade.", notifySchema(allowTargets), func(ctx context.Context, req *protocol.CallToolRequest) *protocol.CallToolResult {
		cfg, err := config.Load(input)
		if err != nil {
			return deliveryResult("notify", delivery.NewResult("", "config", "invalid_config", "could not load notification configuration; run tell config check"))
		}
		args, failure := readArguments(req, notifyFields(cfg.MCP.AgentTargets), notifyOverrideFields, cfg.MCP.AgentTargets)
		if failure != nil {
			return failureResult("notify", cfg.Notify.Via, failure)
		}
		cfg, err = config.Load(inputWithOverrides(input, args, "notify"))
		if err != nil {
			return deliveryResult("notify", delivery.NewResult("", "config", "invalid_config", "could not load notification configuration; run tell config check"))
		}
		if hasOverrides(args, notifyOverrideFields) && !cfg.MCP.AgentTargets {
			return deliveryResult("notify", delivery.NewResult(cfg.Notify.Via, "usage", "agent_targets_disabled", "route overrides require mcp.agent_targets: true in personal configuration"))
		}
		message, failure := notifyMessage(args, cfg)
		if failure != nil {
			return failureResult("notify", cfg.Notify.Via, failure)
		}
		return deliveryResult("notify", delivery.Notify(ctx, cfg, message, stderr))
	})

	addTool(server, "send", "Send one message to the configured agent recipient; accepted means dispatched, not completed.", sendSchema(allowTargets), func(ctx context.Context, req *protocol.CallToolRequest) *protocol.CallToolResult {
		cfg, err := config.Load(input)
		if err != nil {
			return deliveryResult("send", delivery.NewResult("", "config", "invalid_config", "could not load send configuration; run tell config check"))
		}
		args, failure := readArguments(req, sendFields(cfg.MCP.AgentTargets), sendOverrideFields, cfg.MCP.AgentTargets)
		if failure != nil {
			return failureResult("send", cfg.Send.Via, failure)
		}
		cfg, err = config.Load(inputWithOverrides(input, args, "send"))
		if err != nil {
			return deliveryResult("send", delivery.NewResult("", "config", "invalid_config", "could not load send configuration; run tell config check"))
		}
		if hasOverrides(args, sendOverrideFields) && !cfg.MCP.AgentTargets {
			return deliveryResult("send", delivery.NewResult(cfg.Send.Via, "usage", "agent_targets_disabled", "route overrides require mcp.agent_targets: true in personal configuration"))
		}
		intent := "inform"
		if value, ok := args["intent"]; ok {
			intent = value
		}
		if intent != "inform" && intent != "request" {
			return deliveryResult("send", delivery.NewResult(cfg.Send.Via, "usage", "invalid_intent", "send intent must be inform or request"))
		}
		message, ok := args["message"]
		if !ok {
			return deliveryResult("send", delivery.NewResult(cfg.Send.Via, "usage", "invalid_input", "send requires one message"))
		}
		result := delivery.Send(ctx, cfg, message, intent)
		if delivery.ValidRecipient(cfg.Send.Via, cfg.Send.To) {
			result.Recipient = cfg.Send.To
		}
		result.Intent = intent
		return deliveryResult("send", result)
	})

	addTool(server, "doctor", "Check configured route readiness without sending a message.", objectSchema(map[string]any{}, nil), func(ctx context.Context, req *protocol.CallToolRequest) *protocol.CallToolResult {
		_, failure := readArguments(req, map[string]struct{}{}, nil, false)
		if failure != nil {
			return doctorFailureResult("usage", "doctor accepts no arguments")
		}
		cfg, err := config.Load(input)
		if err != nil {
			return doctorFailureResult("config", "could not load configuration; run tell config check")
		}
		routes := delivery.Doctor(ctx, cfg)
		stepCount := len(cfg.Notify.Steps)
		if stepCount == 0 {
			stepCount = 1
		}
		stepCount = min(stepCount, len(routes))
		steps := append([]delivery.Readiness(nil), routes[:stepCount]...)
		ready := false
		for _, step := range steps {
			ready = ready || step.Ready
		}
		selectedSend := ""
		if cfg.Send.Via != "" || cfg.Send.To != "" {
			selectedSend = "send/" + cfg.Send.Via
			ready = ready && routes[len(routes)-1].Ready
		}
		report := doctorReport{SchemaVersion: 1, Selected: cfg.Notify.Via, SelectedSend: selectedSend, Ready: ready, Steps: steps, Routes: routes}
		return &protocol.CallToolResult{
			Content:           []protocol.Content{&protocol.TextContent{Text: doctorSummary(report)}},
			StructuredContent: report,
			IsError:           !report.Ready,
		}
	}, &protocol.ToolAnnotations{ReadOnlyHint: true})
	// Tool discovery must reflect the same live permission as tool execution.
	// Copy SDK-owned tool descriptors rather than mutating shared registrations.
	server.AddReceivingMiddleware(func(next protocol.MethodHandler) protocol.MethodHandler {
		return func(ctx context.Context, method string, req protocol.Request) (protocol.Result, error) {
			result, err := next(ctx, method, req)
			if err != nil || method != "tools/list" {
				return result, err
			}
			cfg, loadErr := config.Load(input)
			allow := loadErr == nil && cfg.MCP.AgentTargets
			listed := *result.(*protocol.ListToolsResult)
			listed.Tools = make([]*protocol.Tool, len(listed.Tools))
			for i, original := range result.(*protocol.ListToolsResult).Tools {
				tool := *original
				switch tool.Name {
				case "notify":
					tool.InputSchema = notifySchema(allow)
				case "send":
					tool.InputSchema = sendSchema(allow)
				}
				listed.Tools[i] = &tool
			}
			return &listed, nil
		}
	})
	return server
}

type doctorReport struct {
	SchemaVersion int                  `json:"schema_version"`
	Selected      string               `json:"selected"`
	SelectedSend  string               `json:"selected_send,omitempty"`
	Ready         bool                 `json:"ready"`
	Steps         []delivery.Readiness `json:"steps"`
	Routes        []delivery.Readiness `json:"routes"`
}

func addTool(server *protocol.Server, name, description string, schema map[string]any, handler func(context.Context, *protocol.CallToolRequest) *protocol.CallToolResult, annotations ...*protocol.ToolAnnotations) {
	var annotation *protocol.ToolAnnotations
	if len(annotations) > 0 {
		annotation = annotations[0]
	}
	server.AddTool(&protocol.Tool{Name: name, Description: description, InputSchema: schema, Annotations: annotation}, func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
		return handler(ctx, req), nil
	})
}

var (
	notifyOverrideFields = map[string]string{
		"via":            "notify.via",
		"herdr_session":  "herdr.session",
		"apprise_tag":    "apprise.tag",
		"apprise_config": "apprise.config",
		"apprise_format": "apprise.format",
	}
	sendOverrideFields = map[string]string{
		"via":           "send.via",
		"to":            "send.to",
		"herdr_session": "herdr.session",
	}
)

func notifyFields(allowTargets bool) map[string]struct{} {
	fields := map[string]struct{}{"message": {}, "title": {}, "type": {}}
	if allowTargets {
		for name := range notifyOverrideFields {
			fields[name] = struct{}{}
		}
	}
	return fields
}

func sendFields(allowTargets bool) map[string]struct{} {
	fields := map[string]struct{}{"message": {}, "intent": {}}
	if allowTargets {
		for name := range sendOverrideFields {
			fields[name] = struct{}{}
		}
	}
	return fields
}

func notifySchema(allowTargets bool) map[string]any {
	properties := map[string]any{
		"message": objectProperty("The message to deliver"),
		"title":   objectProperty("Optional notification title"),
		"type":    enumProperty("Optional notification type", "info", "success", "warning", "failure"),
	}
	if allowTargets {
		properties["via"] = enumProperty("Optional configured notification route", "stderr", "desktop", "apprise", "command", "herdr")
		properties["herdr_session"] = objectProperty("Optional Herdr session selector")
		properties["apprise_tag"] = objectProperty("Optional Apprise tag selector")
		properties["apprise_config"] = objectProperty("Optional local Apprise config file selector")
		properties["apprise_format"] = enumProperty("Optional Apprise body format selector", "text", "markdown", "html")
	}
	return objectSchema(properties, []string{"message"})
}

func sendSchema(allowTargets bool) map[string]any {
	properties := map[string]any{
		"message": objectProperty("The message to deliver"),
		"intent":  enumProperty("Optional message intent", "inform", "request"),
	}
	if allowTargets {
		properties["via"] = enumProperty("Optional configured send route", "hcom", "herdr")
		properties["to"] = objectProperty("Optional exact recipient selector")
		properties["herdr_session"] = objectProperty("Optional Herdr session selector")
	}
	return objectSchema(properties, []string{"message"})
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func objectProperty(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func enumProperty(description string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": description, "enum": values}
}

// readArguments implements the allowlist itself because the low-level SDK API
// intentionally leaves argument validation to the application.
func readArguments(req *protocol.CallToolRequest, allowed map[string]struct{}, overrides map[string]string, allowOverrides bool) (map[string]string, *delivery.Result) {
	if req == nil || req.Params == nil {
		return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be an object")
	}
	raw := req.Params.Arguments
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be an object")
	}
	values := make(map[string]string)
	seen := make(map[string]struct{})
	for decoder.More() {
		token, err := decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok {
			return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be an object")
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must not repeat fields")
		}
		seen[name] = struct{}{}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be valid JSON")
		}
		if _, known := allowed[name]; !known {
			if overrides != nil && containsField(overrides, name) {
				reason := "invalid_override"
				message := "tool argument is not permitted"
				if !allowOverrides {
					reason = "agent_targets_disabled"
					message = "route overrides require mcp.agent_targets: true in personal configuration"
				}
				return nil, delivery.NewResult("", "usage", reason, message)
			}
			return nil, delivery.NewResult("", "usage", "invalid_override", "tool argument is not permitted")
		}
		if len(bytes.TrimSpace(value)) == 0 || bytes.TrimSpace(value)[0] != '"' {
			return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be strings")
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be strings")
		}
		values[name] = text
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must be a valid object")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, delivery.NewResult("", "usage", "invalid_input", "tool arguments must contain one object")
	}
	return values, nil
}

func containsField(fields map[string]string, name string) bool {
	_, ok := fields[name]
	return ok
}

func inputWithOverrides(input config.Input, args map[string]string, tool string) config.Input {
	if len(args) == 0 {
		return input
	}
	input.Overrides = cloneOverrides(input.Overrides)
	overrides := notifyOverrideFields
	if tool == "send" {
		overrides = sendOverrideFields
	}
	for name, key := range overrides {
		if value, ok := args[name]; ok {
			input.Overrides[key] = config.Value{Text: value, Source: "mcp:" + tool}
		}
	}
	return input
}

func cloneOverrides(overrides map[string]config.Value) map[string]config.Value {
	result := make(map[string]config.Value, len(overrides)+3)
	for key, value := range overrides {
		result[key] = value
	}
	return result
}

func hasOverrides(args map[string]string, overrides map[string]string) bool {
	for name := range overrides {
		if _, ok := args[name]; ok {
			return true
		}
	}
	return false
}

func notifyMessage(args map[string]string, cfg config.Config) (delivery.Message, *delivery.Result) {
	body, ok := args["message"]
	if !ok {
		return delivery.Message{}, delivery.NewResult(cfg.Notify.Via, "usage", "invalid_input", "notify requires one message")
	}
	if strings.TrimSpace(body) == "" {
		return delivery.Message{}, delivery.NewResult(cfg.Notify.Via, "config", "empty_message", "notification message must not be empty")
	}
	title := args["title"]
	if _, ok := args["title"]; !ok {
		title = filepath.Base(cfg.Project)
	}
	kind := args["type"]
	if _, ok := args["type"]; !ok {
		kind = "info"
	}
	switch kind {
	case "info", "success", "warning", "failure":
	default:
		return delivery.Message{}, delivery.NewResult(cfg.Notify.Via, "usage", "invalid_type", "notification type must be info, success, warning, or failure")
	}
	return delivery.Message{Title: title, Body: body, Type: kind}, nil
}

func deliveryResult(tool string, result *delivery.Result) *protocol.CallToolResult {
	text := fmt.Sprintf("%s outcome=%s route=%s", tool, result.Outcome, result.Route)
	if result.Failure != nil {
		text += ": " + result.Failure.Message
	}
	return &protocol.CallToolResult{
		Content:           []protocol.Content{&protocol.TextContent{Text: text}},
		StructuredContent: result,
		IsError:           result.ExitCode() != 0,
	}
}

func failureResult(tool, route string, failure *delivery.Result) *protocol.CallToolResult {
	if failure.Route == "" {
		failure.Route = route
	}
	return deliveryResult(tool, failure)
}

func doctorFailureResult(code, message string) *protocol.CallToolResult {
	result := map[string]any{"schema_version": 1, "error": map[string]string{"code": code, "message": message}}
	return &protocol.CallToolResult{
		Content:           []protocol.Content{&protocol.TextContent{Text: "doctor: " + message}},
		StructuredContent: result,
		IsError:           true,
	}
}

func doctorSummary(report doctorReport) string {
	if report.Ready {
		return "Configured routes are ready."
	}
	return "One or more configured routes are not ready."
}
