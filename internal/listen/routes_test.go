package listen

import (
	"encoding/json"
	"testing"

	"github.com/brettinternet/tell/internal/config"
)

func TestRouteMatchingTypedExactPredicates(t *testing.T) {
	prefix := "inc-"
	route := config.Route{
		Name: "incident", Source: "slack", Type: "channel_created",
		Match: map[string]config.Predicate{
			"event.channel.name": {Prefix: &prefix},
			"repository.id":      {Value: json.Number("9007199254740993")},
			"active":             {Value: true},
			"count":              {Value: json.Number("1.0")},
			"zero":               {Value: json.Number("-0")},
		},
	}
	base := Envelope{
		Source: "slack", Type: "channel_created",
		Data: map[string]json.RawMessage{
			"event":      json.RawMessage(`{"channel":{"name":"inc-api"}}`),
			"repository": json.RawMessage(`{"id":9007199254740993}`),
			"active":     json.RawMessage(`true`),
			"count":      json.RawMessage(`1`),
			"zero":       json.RawMessage(`0`),
		},
	}
	if got := Matches([]config.Route{route}, base); len(got) != 1 || got[0].Name != "incident" {
		t.Fatalf("exact nested AND predicates did not match: %v", got)
	}

	for name, mutate := range map[string]func(*Envelope){
		"rounded-large-integer": func(e *Envelope) { e.Data["repository"] = json.RawMessage(`{"id":9007199254740992}`) },
		"type-mismatch":         func(e *Envelope) { e.Data["repository"] = json.RawMessage(`{"id":"9007199254740993"}`) },
		"missing-path":          func(e *Envelope) { delete(e.Data, "active") },
		"null-path":             func(e *Envelope) { e.Data["active"] = json.RawMessage(`null`) },
		"non-object-traversal":  func(e *Envelope) { e.Data["event"] = json.RawMessage(`[]`) },
		"prefix-mismatch":       func(e *Envelope) { e.Data["event"] = json.RawMessage(`{"channel":{"name":7}}`) },
		"wrong-source":          func(e *Envelope) { e.Source = "github" },
	} {
		t.Run(name, func(t *testing.T) {
			event := base
			event.Data = make(map[string]json.RawMessage, len(base.Data))
			for key, value := range base.Data {
				event.Data[key] = append(json.RawMessage(nil), value...)
			}
			mutate(&event)
			if got := Matches([]config.Route{route}, event); len(got) != 0 {
				t.Fatalf("mismatched predicate matched: %v", got)
			}
		})
	}
}
