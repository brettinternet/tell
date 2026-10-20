package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/brettinternet/tell/internal/config"
	deliverypkg "github.com/brettinternet/tell/internal/delivery"
)

const nativeTemplateLimit = 256 * 1024

type renderedAction struct {
	cfg     config.Config
	notify  deliverypkg.Message
	message string
}

func renderAction(ctx context.Context, cfg config.Config, route config.Route, event Envelope) (renderedAction, string) {
	var rendered renderedAction
	rendered.cfg = cfg
	rendered.cfg.Origins = maps.Clone(cfg.Origins)
	used := 0
	values := make(map[string]templateValueResult)
	if reason := contextReason(ctx); reason != "" {
		return renderedAction{}, reason
	}
	switch {
	case route.Notify != nil:
		action := route.Notify
		title := filepath.Base(cfg.Project)
		var reason string
		if action.TitleSet {
			title, reason = renderTemplate(ctx, action.Title, event, &used, values)
		} else if len(title) > nativeTemplateLimit-used {
			reason = "template_size"
		} else {
			used += len(title)
		}
		if reason != "" {
			return renderedAction{}, reason
		}
		message, reason := renderTemplate(ctx, action.Message, event, &used, values)
		if reason != "" {
			return renderedAction{}, reason
		}
		kind := action.Type
		if kind == "" {
			kind = "info"
		}
		if action.Via != "" {
			rendered.cfg.Notify.Via = action.Via
			rendered.cfg.Notify.Steps = nil
		}
		if action.HerdrSession != "" {
			rendered.cfg.Herdr.Session = action.HerdrSession
			if rendered.cfg.Origins == nil {
				rendered.cfg.Origins = make(map[string]string)
			}
			rendered.cfg.Origins["herdr.session"] = "flag:listen-action"
		}
		if action.AppriseTag != "" {
			rendered.cfg.Apprise.Tag = action.AppriseTag
			if rendered.cfg.Origins == nil {
				rendered.cfg.Origins = make(map[string]string)
			}
			rendered.cfg.Origins["apprise.tag"] = "flag:listen-action"
		}
		rendered.notify = deliverypkg.Message{Title: title, Body: message, Type: kind}
	case route.Send != nil:
		action := route.Send
		message, reason := renderTemplate(ctx, action.Message, event, &used, values)
		if reason != "" {
			return renderedAction{}, reason
		}
		if action.Via != "" {
			rendered.cfg.Send.Via = action.Via
		}
		if action.To != "" {
			rendered.cfg.Send.To = action.To
		}
		if action.HerdrSession != "" {
			rendered.cfg.Herdr.Session = action.HerdrSession
		}
		rendered.message = message
	default:
		return renderedAction{}, ""
	}
	return rendered, ""
}

// ValidateAction performs the same template and message preparation checks as
// dispatch, but never resolves a provider, probes a target, or sends anything.
func ValidateAction(ctx context.Context, cfg config.Config, route config.Route, event Envelope) (outcome, reason string) {
	rendered, reason := renderAction(ctx, cfg, route, event)
	if reason != "" {
		return "not_sent", reason
	}
	switch {
	case route.Notify != nil:
		_, _, reason = deliverypkg.PrepareMessage(rendered.notify, rendered.cfg.Secrets.Policy)
	case route.Send != nil:
		_, _, reason = deliverypkg.PrepareSendMessage(rendered.cfg, rendered.message, "inform", event.Source, event.ID)
	}
	if reason != "" {
		return "config", reason
	}
	if reason := contextReason(ctx); reason != "" {
		return "not_sent", reason
	}
	return "dry_run", ""
}

type templateValueResult struct {
	value string
	ok    bool
}

func renderTemplate(ctx context.Context, source string, event Envelope, used *int, values map[string]templateValueResult) (string, string) {
	if reason := contextReason(ctx); reason != "" {
		return "", reason
	}
	parts, err := config.ParseTemplate(source)
	if err != nil {
		return "", "template_value"
	}
	if reason := contextReason(ctx); reason != "" {
		return "", reason
	}
	var rendered strings.Builder
	for _, part := range parts {
		if reason := contextReason(ctx); reason != "" {
			return "", reason
		}
		value := part.Text
		if part.Path != "" {
			cached, exists := values[part.Path]
			if !exists {
				cached.value, cached.ok = templateValue(event, part.Path)
				values[part.Path] = cached
			}
			if reason := contextReason(ctx); reason != "" {
				return "", reason
			}
			value = cached.value
			if !cached.ok {
				return "", "template_value"
			}
		}
		if len(value) > nativeTemplateLimit-*used {
			return "", "template_size"
		}
		*used += len(value)
		rendered.WriteString(value)
	}
	return rendered.String(), ""
}

func contextReason(ctx context.Context) string {
	if err := ctx.Err(); err != nil {
		if err == context.DeadlineExceeded {
			return "timeout"
		}
		return "cancelled"
	}
	return ""
}

func templateValue(event Envelope, path string) (string, bool) {
	if raw, ok := envelopeValue(event, path); ok {
		return scalarTemplateValue(raw)
	}
	if !strings.HasPrefix(path, "data.") {
		return "", false
	}
	value, ok := pathValue(event.Data, strings.TrimPrefix(path, "data."))
	if !ok {
		return "", false
	}
	return scalarTemplateValue(value)
}

func envelopeValue(event Envelope, path string) (json.RawMessage, bool) {
	switch path {
	case "schema_version":
		return json.RawMessage(strconv.Itoa(event.SchemaVersion)), true
	case "source":
		return mustJSON(event.Source), true
	case "type":
		return mustJSON(event.Type), true
	case "id":
		return mustJSON(event.ID), true
	case "received_at":
		return mustJSON(event.ReceivedAt), true
	default:
		return nil, false
	}
}

func mustJSON(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func scalarTemplateValue(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", false
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return "", false
	}
	switch scalar := value.(type) {
	case string:
		return scalar, true
	case bool:
		return strconv.FormatBool(scalar), true
	case json.Number:
		return scalar.String(), true
	default:
		return "", false
	}
}
