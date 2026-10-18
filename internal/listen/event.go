// Package listen receives transient events; it owns no durable state or retries of actions.
package listen

import (
	"bytes"
	"container/list"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const rawLimit = 1 << 20
const envelopeLimit = 256 << 10
const ingressLimit = 64

// errIgnored marks valid non-event payloads, such as interactive callbacks.
var errIgnored = errors.New("ignored")

type Envelope struct {
	SchemaVersion int                        `json:"schema_version"`
	Source        string                     `json:"source"`
	Type          string                     `json:"type"`
	ID            string                     `json:"id"`
	ReceivedAt    string                     `json:"received_at"`
	Data          map[string]json.RawMessage `json:"data"`
}

func metadata(s string, limit int) bool {
	if s == "" || len(s) > limit || !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if unicode.IsControl(c) {
			return false
		}
	}
	return true
}

// Reject duplicate keys at every depth before decoding identities. Limit nesting
// to keep malicious deeply nested objects from exhausting the parser stack.
func object(raw []byte) (map[string]json.RawMessage, error) {
	if len(raw) > rawLimit || !utf8.Valid(raw) {
		return nil, errors.New("invalid_event")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := jsonValue(d, 0); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("invalid_json")
	}
	var result map[string]json.RawMessage
	if json.Unmarshal(raw, &result) != nil || result == nil {
		return nil, errors.New("invalid_object")
	}
	return result, nil
}
func jsonValue(d *json.Decoder, depth int) error {
	if depth > 64 {
		return errors.New("invalid_json")
	}
	token, err := d.Token()
	if err != nil {
		return errors.New("invalid_json")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return errors.New("invalid_json")
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("invalid_json")
			}
			seen[name] = true
			if err := jsonValue(d, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := jsonValue(d, depth+1); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid_json")
	}
	_, err = d.Token()
	if err != nil {
		return errors.New("invalid_json")
	}
	return nil
}

// Decode only the transport object's keys before acknowledging. Payload
// duplicate keys are rejected separately, after receipt of its unambiguous ID.
func frame(raw []byte) (map[string]json.RawMessage, error) {
	if len(raw) > rawLimit || !utf8.Valid(raw) {
		return nil, errors.New("invalid_json")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid_json")
	}
	result := map[string]json.RawMessage{}
	for d.More() {
		token, err = d.Token()
		name, ok := token.(string)
		if err != nil || !ok || result[name] != nil {
			return nil, errors.New("invalid_json")
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, errors.New("invalid_json")
		}
		result[name] = value
	}
	if _, err = d.Token(); err != nil {
		return nil, errors.New("invalid_json")
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("invalid_json")
	}
	return result, nil
}

func field(m map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(m[key], &s)
	return s
}
func event(payload json.RawMessage, now time.Time) (Envelope, []byte, error) {
	p, err := object(payload)
	if err != nil {
		return Envelope{}, nil, err
	}
	if field(p, "type") != "event_callback" {
		return Envelope{}, nil, errIgnored
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(p["event"], &nested) != nil {
		return Envelope{}, nil, errors.New("invalid_event")
	}
	e := Envelope{SchemaVersion: 1, Source: "slack", Type: field(nested, "type"), ID: field(p, "event_id"), ReceivedAt: now.UTC().Format("2006-01-02T15:04:05.000Z"), Data: p}
	if !metadata(e.ID, 256) || !metadata(e.Type, 128) {
		return Envelope{}, nil, errors.New("invalid_identity")
	}
	delete(p, "token")
	encoded, err := json.Marshal(e)
	if err != nil || len(encoded) > envelopeLimit {
		return Envelope{}, nil, errors.New("event_size")
	}
	return e, append(encoded, '\n'), nil
}

type identity struct{ source, id string }
type reservation struct {
	key identity
	at  time.Time
}
type admission struct {
	mu       sync.Mutex
	ids      map[identity]*list.Element
	order    *list.List
	events   chan []byte
	capacity int
	ttl      time.Duration
}

func newAdmission() *admission {
	return &admission{ids: map[identity]*list.Element{}, order: list.New(), events: make(chan []byte, ingressLimit), capacity: 10000, ttl: 10 * time.Minute}
}
func (a *admission) admit(e Envelope, data []byte, now time.Time) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for first := a.order.Front(); first != nil; first = a.order.Front() {
		if now.Sub(first.Value.(reservation).at) < a.ttl {
			break
		}
		a.remove(first)
	}
	key := identity{e.Source, e.ID}
	if a.ids[key] != nil {
		return "duplicate"
	}
	select {
	case a.events <- data:
	default:
		return "not_sent/busy"
	}
	if a.order.Len() == a.capacity {
		a.remove(a.order.Front())
	}
	a.ids[key] = a.order.PushBack(reservation{key, now})
	return "accepted"
}
func (a *admission) remove(element *list.Element) {
	delete(a.ids, element.Value.(reservation).key)
	a.order.Remove(element)
}
