package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
	"github.com/coder/websocket"
)

type capture struct {
	sync.Mutex
	bytes.Buffer
}

func (c *capture) Write(p []byte) (int, error) { c.Lock(); defer c.Unlock(); return c.Buffer.Write(p) }
func (c *capture) text() string                { c.Lock(); defer c.Unlock(); return c.Buffer.String() }
func fixtureTransport(server *httptest.Server) transport {
	tr := production()
	tr.endpoint = server.URL + "/open"
	tr.client = server.Client()
	tr.client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	tr.dial = func(ctx context.Context, address string) (*websocket.Conn, error) {
		c, _, err := websocket.Dial(ctx, address, nil)
		return c, err
	}
	tr.wait = func(ctx context.Context, _ time.Duration) bool { return ctx.Err() == nil }
	return tr
}
func delivery(id string) string {
	return fmt.Sprintf(`{"envelope_id":"ack-%s","type":"events_api","payload":{"type":"event_callback","event_id":%q,"token":"legacy-secret","event":{"type":"channel_created","text":"private-body"}}}`, id, id)
}

// Real HTTP/WebSocket exchange catches credential forwarding, missed acks on
// ignored/duplicate/bad payloads, lost dedup on reconnect, and ticket leakage.
func TestSlackProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var opens atomic.Int32
	var server *httptest.Server
	delivered := make(chan struct{})
	out, diag := &capture{}, &capture{}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open" {
			if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer app-secret" {
				t.Error("missing app authentication")
			}
			n := opens.Add(1)
			fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+fmt.Sprintf("/socket?ticket=private-ticket-%d", n))
			return
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("app credential reached websocket")
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		for _, frame := range []string{delivery("Ev1"), delivery("Ev1"), `{"envelope_id":"ignored","type":"interactive","payload":{}}`, `{"envelope_id":"bad","type":"events_api","payload":{"type":"event_callback","event_id":42}}`, `{"envelope_id":"duplicate-payload","type":"events_api","payload":{"type":"event_callback","event_id":"a","event_id":"b","event":{"type":"x"}}}`} {
			if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
				t.Error(err)
				return
			}
			_, ack, err := conn.Read(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			var got map[string]string
			if json.Unmarshal(ack, &got) != nil || got["envelope_id"] == "" || len(got) != 1 {
				t.Errorf("bad ack: %s", ack)
			}
		}
		if opens.Load() == 1 {
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"disconnect","reason":"refresh_requested"}`))
			return
		}
		close(delivered)
		<-ctx.Done()
	}))
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, SlackOptions{Token: "app-secret", Timeout: time.Second, Out: out, Err: diag}, fixtureTransport(server))
	}()
	select {
	case <-delivered:
	case <-ctx.Done():
		t.Fatal("protocol did not complete")
	}
	// Wait for the actual export rather than relying on scheduler timing.
	deadline := time.After(time.Second)
	for !strings.Contains(out.text(), "Ev1") {
		select {
		case <-deadline:
			t.Fatal("missing export")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if opens.Load() != 2 {
		t.Fatalf("opens=%d", opens.Load())
	}
	text := out.text()
	if strings.Count(text, "\n") != 1 || strings.Contains(text, "legacy-secret") || !strings.Contains(text, "private-body") {
		t.Fatalf("export: %s", text)
	}
	var envelope Envelope
	if json.Unmarshal([]byte(text), &envelope) != nil || envelope.Source != "slack" || envelope.ID != "Ev1" || envelope.Type != "channel_created" || envelope.SchemaVersion != 1 {
		t.Fatal(text)
	}
	for _, secret := range []string{"private-body", "legacy-secret", "app-secret", "private-ticket"} {
		if strings.Contains(diag.text(), secret) {
			t.Fatalf("diagnostic leaked %s", secret)
		}
	}
}

// Native stderr notifications share the listener's bounded single-writer path,
// so concurrent delivery workers cannot race a bytes.Buffer or block the ack.
func TestNativeStderrOutputUsesBoundedListenerWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var server *httptest.Server
	acknowledged := make(chan struct{})
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open" {
			fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/socket")
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		if err := conn.Write(ctx, websocket.MessageText, []byte(delivery("Ev1"))); err != nil {
			t.Error(err)
			return
		}
		if _, _, err := conn.Read(ctx); err != nil {
			t.Error(err)
			return
		}
		close(acknowledged)
		<-ctx.Done()
	}))
	defer server.Close()

	cfg := config.Config{Project: "/project", Timeout: "3s", Notify: config.Notify{Via: "stderr"}, Secrets: config.Secrets{Policy: "redact"}}
	for i := range 4 {
		cfg.Listen.Routes = append(cfg.Listen.Routes, config.Route{
			Name: fmt.Sprintf("notify-%d", i), Source: "slack", Type: "channel_created",
			Notify: &config.NotifyAction{Message: fmt.Sprintf("intentional-%d-${id}", i)},
		})
	}
	executor := NewExecutor(cfg, currentDirectory(t))
	var out, diag capture
	results := make(chan string, 4)
	opts := SlackOptions{Token: "fixture-token", Timeout: time.Second, Out: &out, Err: &diag}
	opts.OnEvent = func(event Envelope, report func(route, outcome, reason string), writer io.Writer) {
		for _, route := range Matches(cfg.Listen.Routes, event) {
			if !executor.SubmitWithWriter(route, event, func(name, outcome, reason string) {
				results <- name + ":" + outcome
			}, writer) {
				report(route.Name, "not_sent", "busy")
			}
		}
	}
	opts.OnStop = func() { executor.Stop(); executor.Wait() }
	done := make(chan error, 1)
	go func() { done <- run(ctx, opts, fixtureTransport(server)) }()
	select {
	case <-acknowledged:
	case <-time.After(3 * time.Second):
		t.Fatal("listener did not acknowledge the event")
	}
	for range 4 {
		select {
		case result := <-results:
			if !strings.HasSuffix(result, ":accepted") {
				t.Fatalf("notification result: %s", result)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("native stderr notifications did not finish")
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if out.text() != "" {
		t.Fatalf("intentional stderr delivery used stdout: %q", out.text())
	}
	for i := range 4 {
		if got := strings.Count(diag.text(), fmt.Sprintf("intentional-%d-Ev1\n", i)); got != 1 {
			t.Fatalf("stderr delivery %d count=%d: %q", i, got, diag.text())
		}
	}
}

type gatedWriter struct {
	started     chan struct{}
	release     chan struct{}
	err         error
	startOnce   sync.Once
	releaseOnce sync.Once
}

func (writer *gatedWriter) Write(data []byte) (int, error) {
	writer.startOnce.Do(func() { close(writer.started) })
	<-writer.release
	if writer.err != nil {
		return 0, writer.err
	}
	return len(data), nil
}

func (writer *gatedWriter) unblock() { writer.releaseOnce.Do(func() { close(writer.release) }) }

func startStderrAction(t *testing.T, stream io.Writer, outputTimeout time.Duration) (*Executor, <-chan [3]string, <-chan error, func()) {
	t.Helper()
	queue := make(chan *outputRequest, 64)
	outputCtx, cancel := context.WithCancel(context.Background())
	failures := make(chan error, 1)
	outputDone := make(chan struct{})
	go func() {
		output(outputCtx, stream, queue, outputTimeout, func(err error) {
			cancel()
			failures <- err
		})
		close(outputDone)
	}()
	cfg := config.Config{Project: "/project", Timeout: "2s", Notify: config.Notify{Via: "stderr"}, Secrets: config.Secrets{Policy: "redact"}}
	executor := NewExecutor(cfg, currentDirectory(t))
	results := make(chan [3]string, 1)
	route := config.Route{Name: "native-stderr", Notify: &config.NotifyAction{Message: "deliver", Via: "stderr"}}
	writer := &queuedWriter{queue: queue, fail: func(err error) { cancel(); failures <- err }, ctx: outputCtx}
	if !executor.SubmitWithWriter(route, Envelope{Source: "slack", Type: "event", ID: "Ev1"}, func(route, outcome, reason string) {
		results <- [3]string{route, outcome, reason}
	}, writer) {
		t.Fatal("native stderr action was not admitted")
	}
	cleanup := func() {
		executor.Stop()
		executor.Wait()
		close(queue)
		cancel()
		<-outputDone
	}
	return executor, results, failures, cleanup
}

// Native stderr actions must wait for the serialized writer's actual result,
// not report acceptance when their bytes are merely queued.
func TestNativeStderrActionWaitsForActualWrite(t *testing.T) {
	for _, tc := range []struct {
		name     string
		writeErr error
		outcome  string
		reason   string
	}{
		{name: "success", outcome: "accepted"},
		{name: "failure", writeErr: io.ErrClosedPipe, outcome: "failed", reason: "io"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stream := &gatedWriter{started: make(chan struct{}), release: make(chan struct{}), err: tc.writeErr}
			_, results, _, cleanup := startStderrAction(t, stream, time.Second)
			defer func() { stream.unblock(); cleanup() }()
			select {
			case <-stream.started:
			case <-time.After(time.Second):
				t.Fatal("stderr writer was not called")
			}
			select {
			case result := <-results:
				t.Fatalf("action reported before write completion: %v", result)
			case <-time.After(20 * time.Millisecond):
			}
			stream.unblock()
			select {
			case result := <-results:
				if result[1] != tc.outcome || result[2] != tc.reason {
					t.Fatalf("action result = %v, want %s/%s", result, tc.outcome, tc.reason)
				}
			case <-time.After(time.Second):
				t.Fatal("stderr action did not receive its write result")
			}
		})
	}
}

// A stalled native-action write is uncertain and releases its worker slot.
func TestStalledNativeStderrActionReleasesWorker(t *testing.T) {
	stream := &gatedWriter{started: make(chan struct{}), release: make(chan struct{})}
	_, results, failures, cleanup := startStderrAction(t, stream, 30*time.Millisecond)
	defer func() { stream.unblock(); cleanup() }()
	select {
	case <-stream.started:
	case <-time.After(time.Second):
		t.Fatal("stderr writer was not called")
	}
	select {
	case result := <-results:
		if result[1] != "uncertain" || result[2] != "output_stalled" {
			t.Fatalf("stalled output action = %v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("stalled writer held its executor slot")
	}
	select {
	case err := <-failures:
		if !strings.Contains(err.Error(), "stalled") {
			t.Fatalf("global output failure = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("stalled output did not fail the listener writer")
	}
}

// A stalled writer failure stops the listener, and queued native actions do
// not keep the executor alive while waiting for output that can no longer run.
func TestStalledNativeStderrActionStopsListener(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var server *httptest.Server
	acknowledged := make(chan struct{})
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open" {
			fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/socket")
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		if err := conn.Write(ctx, websocket.MessageText, []byte(delivery("stalled"))); err != nil {
			t.Error(err)
			return
		}
		if _, _, err := conn.Read(ctx); err != nil {
			t.Error(err)
			return
		}
		close(acknowledged)
		<-ctx.Done()
	}))
	defer func() { cancel(); server.Close() }()

	stream := &gatedWriter{started: make(chan struct{}), release: make(chan struct{})}
	cfg := config.Config{Project: "/project", Timeout: "2s", Notify: config.Notify{Via: "stderr"}, Secrets: config.Secrets{Policy: "redact"}}
	route := config.Route{Name: "native-stderr", Source: "slack", Type: "channel_created", Notify: &config.NotifyAction{Message: "deliver", Via: "stderr"}}
	cfg.Listen.Routes = []config.Route{route}
	executor := NewExecutor(cfg, currentDirectory(t))
	results := make(chan [3]string, 1)
	var out capture
	options := SlackOptions{Token: "fixture-token", Timeout: time.Second, Out: &out, Err: stream}
	options.OnEvent = func(event Envelope, report func(string, string, string), writer io.Writer) {
		if !executor.SubmitWithWriter(route, event, func(name, outcome, reason string) {
			results <- [3]string{name, outcome, reason}
		}, writer) {
			report(route.Name, "not_sent", "busy")
		}
	}
	options.OnStop = func() { executor.Stop(); executor.Wait() }
	tr := fixtureTransport(server)
	tr.outputTimeout = 30 * time.Millisecond
	done := make(chan error, 1)
	go func() { done <- run(ctx, options, tr) }()
	select {
	case <-acknowledged:
	case <-ctx.Done():
		t.Fatal("listener did not acknowledge the event")
	}
	select {
	case <-stream.started:
	case <-ctx.Done():
		t.Fatal("native stderr output did not start")
	}
	select {
	case result := <-results:
		if result[1] != "not_sent" || result[2] != "cancelled" {
			t.Fatalf("queued action after listener output failure = %v", result)
		}
	case <-ctx.Done():
		t.Fatal("stalled writer held its executor slot")
	}
	stream.unblock()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stalled") {
			t.Fatalf("listener did not stop on output failure: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("listener/executor did not drain after writer failure")
	}
}

// A duplicated admitted envelope must not repeat either successful sibling action.
func TestDuplicateEventDoesNotRepeatMatchingActions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var server *httptest.Server
	ready := make(chan struct{})
	eventFrame := `{"envelope_id":"ack","type":"events_api","payload":{"type":"event_callback","event_id":"same-id","event":{"type":"channel_created","channel":{"id":"C1","name":"inc-api"},"text":"private-event-body"}}}`
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open" {
			fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/socket")
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		for range 2 {
			if err := conn.Write(ctx, websocket.MessageText, []byte(eventFrame)); err != nil {
				t.Error(err)
				return
			}
			if _, _, err := conn.Read(ctx); err != nil {
				t.Error(err)
				return
			}
		}
		close(ready)
		<-ctx.Done()
	}))
	defer server.Close()

	recordDir := t.TempDir()
	startDir := t.TempDir()
	release := filepath.Join(t.TempDir(), "release")
	t.Setenv("FIXTURE_RECORD_DIR", recordDir)
	t.Setenv("FIXTURE_START_DIR", startDir)
	t.Setenv("FIXTURE_RELEASE_FILE", release)
	prefix := "inc-"
	cfg := config.Config{
		Project: "/project", Timeout: "3s",
		Listen: config.Listen{Slack: &config.Slack{AppTokenEnv: "LISTENER_CREDENTIAL"}},
	}
	cfg.Listen.Routes = []config.Route{
		{Name: "first", Source: "slack", Type: "channel_created", Match: map[string]config.Predicate{"event.channel.name": {Prefix: &prefix}}, Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child"}},
		{Name: "second", Source: "slack", Type: "channel_created", Match: map[string]config.Predicate{"event.channel.id": {Value: "C1"}}, Run: []string{os.Args[0], "-test.run=^TestCommandFixtureProcess$", "--", "--fixture-child"}},
		{Name: "native-notify", Source: "slack", Type: "channel_created", Notify: &config.NotifyAction{Message: "native-${id}", Via: "stderr"}},
	}
	executor := NewExecutor(cfg, currentDirectory(t))
	var out, diag capture
	opts := SlackOptions{Token: "fixture-token", Timeout: time.Second, Out: &out, Err: &diag}
	opts.OnEvent = func(event Envelope, report func(route, outcome, reason string), deliveryWriter io.Writer) {
		for _, route := range Matches(cfg.Listen.Routes, event) {
			if !executor.SubmitWithWriter(route, event, report, deliveryWriter) {
				report(route.Name, "not_sent", "busy")
			}
		}
	}
	opts.OnStop = func() { executor.Stop(); executor.Wait() }
	done := make(chan error, 1)
	go func() { done <- run(ctx, opts, fixtureTransport(server)) }()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("Slack fixture did not acknowledge both deliveries")
	}
	waitFor(t, func() bool {
		entries, err := os.ReadDir(startDir)
		return err == nil && len(entries) == 2
	})
	if _, err := os.Stat(release); !os.IsNotExist(err) {
		t.Fatal("slow actions were released before both event acknowledgments")
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not drain action results")
	}
	for _, name := range []string{"first", "second"} {
		data, err := os.ReadFile(filepath.Join(recordDir, name))
		if err != nil || strings.Count(string(data), "called") != 1 {
			t.Fatalf("route %s executed other than once: %q %v", name, data, err)
		}
	}
	if strings.Contains(diag.text(), "private-event-body") {
		t.Fatal("event body escaped into diagnostics")
	}
	if got := strings.Count(diag.text(), "native-same-id\n"); got != 1 {
		t.Fatalf("duplicate delivery repeated native sibling: count=%d diagnostics=%q", got, diag.text())
	}
}

// Malformed identities, duplicate nested keys and encoded expansion must never
// become admitted events, even though their transport envelope can be acked.
func TestEventValidation(t *testing.T) {
	for _, raw := range []string{
		`{"type":"event_callback","event_id":"a","event_id":"b","event":{"type":"x"}}`,
		`{"type":"event_callback","event_id":"a","event":{"type":"x","type":"y"}}`,
		`{"type":"event_callback","event_id":"a\n","event":{"type":"x"}}`,
		`{"type":"event_callback","event_id":4,"event":{"type":"x"}}`,
		`{"type":"event_callback","event_id":"a","event":{"type":""}}`,
		`{"type":"event_callback","event_id":"a","event":{"type":"x"},"body":"` + strings.Repeat("<", envelopeLimit) + `"}`,
		strings.Repeat(" ", rawLimit+1),
		`{} {}`,
	} {
		if _, _, err := event([]byte(raw), time.Now()); err == nil {
			t.Fatalf("accepted invalid event (%d bytes)", len(raw))
		}
	}
}

// Atomic admission prevents concurrent redeliveries executing twice; full
// queues must not reserve IDs, duplicates must not extend TTL, and eviction
// must remain oldest-admission first and source scoped.
func TestAdmission(t *testing.T) {
	a := newAdmission()
	now := time.Now()
	e := Envelope{Source: "slack", ID: "same"}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() {
			if a.admit(e, nil, now) == "accepted" {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatal(accepted.Load())
	}
	<-a.events
	if a.admit(e, nil, now.Add(9*time.Minute)) != "duplicate" || a.admit(e, nil, now.Add(10*time.Minute)) != "accepted" {
		t.Fatal("TTL refreshed by duplicate")
	}
	<-a.events
	e.Source = "other"
	if a.admit(e, nil, now.Add(10*time.Minute)) != "accepted" {
		t.Fatal("not source scoped")
	}
	<-a.events
	a = newAdmission()
	a.capacity = 2
	for _, id := range []string{"a", "b", "c", "a"} {
		if a.admit(Envelope{Source: "slack", ID: id}, nil, now) != "accepted" {
			t.Fatal("oldest not evicted")
		}
		<-a.events
	}
	a = newAdmission()
	for i := range ingressLimit {
		if a.admit(Envelope{Source: "slack", ID: fmt.Sprint(i)}, nil, now) != "accepted" {
			t.Fatal("early overflow")
		}
	}
	e.ID = "overflow"
	if a.admit(e, nil, now) != "not_sent/busy" {
		t.Fatal("unbounded ingress")
	}
	<-a.events
	if a.admit(e, nil, now) != "accepted" {
		t.Fatal("drop reserved identity")
	}
}

// HTTP retries and fatal errors must expose only fixed reasons, and redirects
// may not forward app credentials. No real Slack endpoint is used.
func TestSetupRecovery(t *testing.T) {
	for _, status := range []int{429, 500, 503, 401} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "7")
				w.WriteHeader(status)
				fmt.Fprint(w, "raw-secret")
			}))
			defer server.Close()
			_, err := open(context.Background(), SlackOptions{Timeout: time.Second}, fixtureTransport(server))
			if err == nil || err.retry != (status != 401) || strings.Contains(err.Error(), "raw-secret") {
				t.Fatalf("%+v", err)
			}
			if status == 429 && err.after != 7*time.Second {
				t.Fatal(err.after)
			}
		})
	}
	for _, code := range []string{"invalid_auth", "raw-secret"} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, `{"ok":false,"error":%q}`, code) }))
		_, err := open(context.Background(), SlackOptions{Timeout: time.Second}, fixtureTransport(server))
		server.Close()
		if err == nil || err.retry || strings.Contains(err.Error(), "raw-secret") {
			t.Fatalf("%+v", err)
		}
	}
	var forwarded atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded.Store(true) }))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
	defer redirect.Close()
	_, err := open(context.Background(), SlackOptions{Token: "app-secret", Timeout: time.Second}, fixtureTransport(redirect))
	if err == nil || forwarded.Load() {
		t.Fatal("followed credential redirect")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if wait(ctx, time.Hour) {
		t.Fatal("backoff ignored cancellation")
	}
	if _, err := production().dial(context.Background(), "ws://example.invalid"); err == nil {
		t.Fatal("plaintext permitted")
	}
}

// A single run must retry transient HTTP/network failures, respect Retry-After,
// then stop on link_disabled rather than spinning or leaking provider data.
func TestWebSocketSetupFailure(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "8")
			w.WriteHeader(status)
		}))
		tr := production()
		// Keep production WSS validation and handshake behavior, trusting only this fixture.
		tr.client.Transport = server.Client().Transport
		_, err := tr.dial(context.Background(), "wss"+strings.TrimPrefix(server.URL, "https"))
		server.Close()
		var setup *setupError
		if !errors.As(err, &setup) || setup.retry != (status == http.StatusTooManyRequests) {
			t.Fatalf("%d: %v", status, err)
		}
		if setup.retry && setup.after != 8*time.Second {
			t.Fatal(setup.after)
		}
	}
}

func TestRecoverySequence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var server *httptest.Server
	var requests atomic.Int32
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open" {
			switch requests.Add(1) {
			case 1:
				w.WriteHeader(503)
			case 2:
				w.Header().Set("Retry-After", "9")
				w.WriteHeader(429)
			default:
				fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/socket?ticket=private-ticket")
			}
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"disconnect","reason":"link_disabled","debug_info":{"host":"private-host"}}`))
		_, _, _ = conn.Read(ctx)
	}))
	defer server.Close()
	tr := fixtureTransport(server)
	var waits []time.Duration
	tr.jitter = func(d time.Duration) time.Duration { return d }
	tr.wait = func(ctx context.Context, d time.Duration) bool { waits = append(waits, d); return ctx.Err() == nil }
	out, diag := &capture{}, &capture{}
	err := run(ctx, SlackOptions{Timeout: time.Second, Out: out, Err: diag}, tr)
	if err == nil || !strings.Contains(err.Error(), "link_disabled") || requests.Load() != 3 {
		t.Fatalf("%v requests=%d", err, requests.Load())
	}
	if len(waits) != 2 || waits[0] != time.Second || waits[1] != 9*time.Second {
		t.Fatalf("waits %v", waits)
	}
	if strings.Contains(diag.text(), "private-") || !strings.Contains(diag.text(), "link_disabled") {
		t.Fatal(diag.text())
	}
	// A refused local socket is retryable, unlike an auth/setup error.
	closed := httptest.NewServer(http.NotFoundHandler())
	tr = fixtureTransport(closed)
	closed.Close()
	_, failure := open(ctx, SlackOptions{Timeout: time.Second}, tr)
	if failure == nil || !failure.retry || failure.reason != "network" {
		t.Fatalf("%v", failure)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

// Broken stdout/stderr must fail instead of silently losing capture. Oversized
// raw frames must disconnect before decoding, without admitting any payload.
func TestOutputAndFrameFailures(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/open" {
					fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/socket")
					return
				}
				conn, err := websocket.Accept(w, r, nil)
				if err != nil {
					return
				}
				defer conn.CloseNow()
				_ = conn.Write(ctx, websocket.MessageText, []byte(delivery("broken")))
				for {
					if _, _, err = conn.Read(ctx); err != nil {
						return
					}
				}
			}))
			defer server.Close()
			opts := SlackOptions{Timeout: time.Second, Out: io.Discard, Err: io.Discard}
			if stream == "stdout" {
				opts.Out = brokenWriter{}
			} else {
				opts.Err = brokenWriter{}
			}
			if err := run(ctx, opts, fixtureTransport(server)); err == nil || !strings.Contains(err.Error(), "output") {
				t.Fatalf("%v", err)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_ = conn.Write(ctx, websocket.MessageText, []byte(strings.Repeat("x", rawLimit+1)))
		_, _, _ = conn.Read(ctx)
	}))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	a := newAdmission()
	if err := connection(ctx, conn, production(), a, func(Envelope, string) {}); err != nil {
		t.Fatal(err)
	}
	if len(a.events) != 0 {
		t.Fatal("oversized frame admitted")
	}
}

type blockingWriter struct{ release chan struct{} }

func (b blockingWriter) Write(p []byte) (int, error) { <-b.release; return len(p), nil }

// A stalled stream must not stall acks or create one goroutine per event.
func TestStalledOutputAcknowledgments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var server *httptest.Server
	acked := make(chan struct{})
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/open" {
			fmt.Fprintf(w, `{"ok":true,"url":%q}`, "ws"+strings.TrimPrefix(server.URL, "http")+"/socket")
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for i := range 100 {
			if conn.Write(ctx, websocket.MessageText, []byte(delivery(fmt.Sprint(i)))) != nil {
				return
			}
			if _, _, err = conn.Read(ctx); err != nil {
				return
			}
		}
		close(acked)
		<-ctx.Done()
	}))
	defer server.Close()
	release := make(chan struct{})
	defer close(release)
	tr := fixtureTransport(server)
	tr.outputTimeout = 500 * time.Millisecond
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, SlackOptions{Timeout: time.Second, Out: blockingWriter{release}, Err: io.Discard}, tr)
	}()
	select {
	case <-acked:
	case <-ctx.Done():
		t.Fatal("acks blocked on stdout")
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stalled") {
			t.Fatalf("%v", err)
		}
	case <-ctx.Done():
		t.Fatal("stalled output did not stop")
	}
}
