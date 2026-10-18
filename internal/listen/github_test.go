package listen

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	githubTestSecret = "dedicated-test-secret"
	githubTestID     = "123e4567-e89b-12d3-a456-426614174000"
)

func githubBody(repositoryID int64, action string) string {
	return fmt.Sprintf(`{"action":%q,"repository":{"id":%d,"name":"private-repository"},"created_at":"1999-01-01T00:00:00Z","pull_request":{"title":"private-body"}}`, action, repositoryID)
}

func githubRequest(body, eventType, deliveryID, secret string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, githubPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-GitHub-Delivery", deliveryID)
	req.Header.Set("X-GitHub-Event", eventType)
	if secret != "" {
		req.Header.Set("X-Hub-Signature-256", githubHMAC(body, secret))
	}
	return req
}

func githubHMAC(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func githubFixtureHandler(a *admission) http.Handler {
	return githubHandler(githubTestSecret, map[int64]bool{42: true}, a, func(Envelope, string) {}, make(chan struct{}, githubReaderLimit))
}

func TestGitHubSignedWebhookEnvelopePingAndReplaySuppression(t *testing.T) {
	a := newAdmission()
	handler := githubFixtureHandler(a)
	body := githubBody(42, "opened")
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, githubRequest(body, "pull_request", githubTestID, githubTestSecret))
	if first.Code != http.StatusAccepted {
		t.Fatalf("accepted response=%d", first.Code)
	}
	var event Envelope
	if err := json.Unmarshal(<-a.events, &event); err != nil {
		t.Fatal(err)
	}
	if event.SchemaVersion != 1 || event.Source != "github" || event.Type != "pull_request" || event.ID != githubTestID || len(event.Data) != 4 {
		t.Fatalf("wrong envelope metadata or body: %+v", event)
	}
	var repository map[string]any
	if err := json.Unmarshal(event.Data["repository"], &repository); err != nil || repository["name"] != "private-repository" {
		t.Fatalf("parsed body changed: %s %v", event.Data["repository"], err)
	}
	if _, found := event.Data["X-Hub-Signature-256"]; found {
		t.Fatal("transport header entered event data")
	}
	serialized, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serialized), githubTestSecret) || event.ReceivedAt == "" {
		t.Fatalf("envelope leaked secret or missed receipt time: %+v", event)
	}

	duplicate := httptest.NewRecorder()
	handler.ServeHTTP(duplicate, githubRequest(body, "pull_request", githubTestID, githubTestSecret))
	changedID := httptest.NewRecorder()
	secondID := "123e4567-e89b-12d3-a456-426614174001"
	handler.ServeHTTP(changedID, githubRequest(body, "pull_request", secondID, githubTestSecret))
	if duplicate.Code != http.StatusOK || changedID.Code != http.StatusOK || len(a.events) != 0 {
		t.Fatalf("duplicate results=%d/%d queued=%d", duplicate.Code, changedID.Code, len(a.events))
	}
	if a.ids[identity{"github", secondID}] != nil {
		t.Fatal("digest duplicate reserved its changed unsigned delivery ID")
	}

	ping := httptest.NewRecorder()
	handler.ServeHTTP(ping, githubRequest(githubBody(42, ""), "ping", secondID, githubTestSecret))
	if ping.Code != http.StatusNoContent || len(a.events) != 0 {
		t.Fatalf("verified ping response=%d queued=%d", ping.Code, len(a.events))
	}

	unrelated := httptest.NewRecorder()
	handler.ServeHTTP(unrelated, githubRequest(githubBody(42, "closed"), "issues", "123e4567-e89b-12d3-a456-426614174002", githubTestSecret))
	if unrelated.Code != http.StatusAccepted || len(a.events) != 1 {
		t.Fatalf("valid unrelated event response=%d queued=%d", unrelated.Code, len(a.events))
	}
}

func TestGitHubSignatureAndRequestValidationStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status int
		mutate func(*http.Request)
	}{
		{name: "missing signature", status: http.StatusUnauthorized, mutate: func(req *http.Request) { req.Header.Del("X-Hub-Signature-256") }},
		{name: "SHA-1 only", status: http.StatusUnauthorized, mutate: func(req *http.Request) {
			req.Header.Del("X-Hub-Signature-256")
			req.Header.Set("X-Hub-Signature", "sha1=deadbeef")
		}},
		{name: "bad signature", status: http.StatusUnauthorized, mutate: func(req *http.Request) { req.Header.Set("X-Hub-Signature-256", "sha256="+strings.Repeat("0", 64)) }},
		{name: "ambiguous signature", status: http.StatusBadRequest, mutate: func(req *http.Request) {
			req.Header.Add("X-Hub-Signature-256", githubHMAC(githubBody(42, "opened"), githubTestSecret))
		}},
		{name: "ambiguous delivery", status: http.StatusBadRequest, mutate: func(req *http.Request) { req.Header.Add("X-GitHub-Delivery", githubTestID) }},
		{name: "comma delivery", status: http.StatusBadRequest, mutate: func(req *http.Request) { req.Header.Set("X-GitHub-Delivery", githubTestID+", other") }},
		{name: "missing event", status: http.StatusBadRequest, mutate: func(req *http.Request) { req.Header.Del("X-GitHub-Event") }},
		{name: "invalid delivery UUID", status: http.StatusBadRequest, mutate: func(req *http.Request) { req.Header.Set("X-GitHub-Delivery", "not-a-uuid") }},
		{name: "invalid event name", status: http.StatusBadRequest, mutate: func(req *http.Request) { req.Header.Set("X-GitHub-Event", "Pull_Request") }},
		{name: "content type", status: http.StatusUnsupportedMediaType, mutate: func(req *http.Request) { req.Header.Set("Content-Type", "text/plain") }},
		{name: "unsupported charset", status: http.StatusUnsupportedMediaType, mutate: func(req *http.Request) { req.Header.Set("Content-Type", "application/json; charset=latin1") }},
		{name: "content encoding", status: http.StatusUnsupportedMediaType, mutate: func(req *http.Request) { req.Header.Set("Content-Encoding", "gzip") }},
		{name: "ambiguous encoding", status: http.StatusUnsupportedMediaType, mutate: func(req *http.Request) {
			req.Header.Set("Content-Encoding", "identity")
			req.Header.Add("Content-Encoding", "identity")
		}},
		{name: "wrong method", status: http.StatusMethodNotAllowed, mutate: func(req *http.Request) { req.Method = http.MethodGet }},
		{name: "wrong path", status: http.StatusNotFound, mutate: func(req *http.Request) { req.URL.Path = "/other" }},
		{name: "malformed JSON", status: http.StatusBadRequest, mutate: func(req *http.Request) {
			req.Body = io.NopCloser(strings.NewReader("{"))
			req.ContentLength = 1
			req.Header.Set("X-Hub-Signature-256", githubHMAC("{", githubTestSecret))
		}},
		{name: "duplicate JSON key", status: http.StatusBadRequest, mutate: func(req *http.Request) {
			body := `{"repository":{"id":42,"id":43}}`
			req.Body = io.NopCloser(strings.NewReader(body))
			req.ContentLength = int64(len(body))
			req.Header.Set("X-Hub-Signature-256", githubHMAC(body, githubTestSecret))
		}},
		{name: "repository outside allowlist", status: http.StatusForbidden, mutate: func(req *http.Request) {
			body := githubBody(43, "opened")
			req.Body = io.NopCloser(strings.NewReader(body))
			req.ContentLength = int64(len(body))
			req.Header.Set("X-Hub-Signature-256", githubHMAC(body, githubTestSecret))
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newAdmission()
			body := githubBody(42, "opened")
			req := githubRequest(body, "pull_request", githubTestID, githubTestSecret)
			test.mutate(req)
			response := httptest.NewRecorder()
			githubFixtureHandler(a).ServeHTTP(response, req)
			if response.Code != test.status || len(a.events) != 0 || len(a.ids) != 0 || len(a.digests) != 0 {
				t.Fatalf("status=%d want=%d queued=%d ids=%d digests=%d", response.Code, test.status, len(a.events), len(a.ids), len(a.digests))
			}
			if response.Body.Len() != 0 {
				t.Fatalf("response contained a body: %q", response.Body.String())
			}
		})
	}
}

func TestGitHubBodyAndEnvelopeLimitsAndShutdownAdmission(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "raw body", body: strings.Repeat("x", rawLimit+1)},
		{name: "encoded envelope", body: `{"repository":{"id":42},"text":"` + strings.Repeat("x", envelopeLimit) + `"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := newAdmission()
			req := githubRequest(test.body, "pull_request", githubTestID, githubTestSecret)
			response := httptest.NewRecorder()
			githubFixtureHandler(a).ServeHTTP(response, req)
			if response.Code != http.StatusRequestEntityTooLarge || len(a.events) != 0 || len(a.ids) != 0 || len(a.digests) != 0 {
				t.Fatalf("status=%d queued=%d ids=%d digests=%d", response.Code, len(a.events), len(a.ids), len(a.digests))
			}
		})
	}

	full := newAdmission()
	for i := range ingressLimit {
		event := Envelope{Source: "slack", ID: fmt.Sprint(i)}
		if full.admit(event, []byte("{}"), time.Now()) != "accepted" {
			t.Fatal("could not fill event ingress fixture")
		}
	}
	body := githubBody(42, "opened")
	req := githubRequest(body, "pull_request", githubTestID, githubTestSecret)
	response := httptest.NewRecorder()
	githubFixtureHandler(full).ServeHTTP(response, req)
	if response.Code != http.StatusServiceUnavailable || full.ids[identity{"github", githubTestID}] != nil || len(full.digests) != 0 {
		t.Fatalf("full ingress status=%d IDs=%d digests=%d", response.Code, len(full.ids), len(full.digests))
	}
	<-full.events
	retry := httptest.NewRecorder()
	githubFixtureHandler(full).ServeHTTP(retry, githubRequest(body, "pull_request", githubTestID, githubTestSecret))
	if retry.Code != http.StatusAccepted {
		t.Fatalf("retry after rejected admission status=%d", retry.Code)
	}

	full.close()
	shutdown := httptest.NewRecorder()
	githubFixtureHandler(full).ServeHTTP(shutdown, githubRequest(body, "pull_request", "123e4567-e89b-12d3-a456-426614174003", githubTestSecret))
	if shutdown.Code != http.StatusServiceUnavailable {
		t.Fatalf("shutdown admission status=%d", shutdown.Code)
	}
}

func TestGitHubConcurrentBodyDigestAndSourceScopedIdentity(t *testing.T) {
	a := newAdmission()
	handler := githubFixtureHandler(a)
	body := githubBody(42, "opened")
	const attempts = 12
	var accepted, duplicates int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("123e4567-e89b-12d3-a456-%012d", i)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, githubRequest(body, "pull_request", id, githubTestSecret))
			mu.Lock()
			switch response.Code {
			case http.StatusAccepted:
				accepted++
			case http.StatusOK:
				duplicates++
			default:
				t.Errorf("concurrent webhook status=%d", response.Code)
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if accepted != 1 || duplicates != attempts-1 || len(a.events) != 1 || len(a.ids) != 1 || len(a.digests) != 1 {
		t.Fatalf("atomic body dedup accepted=%d duplicate=%d queue=%d IDs=%d digest=%d", accepted, duplicates, len(a.events), len(a.ids), len(a.digests))
	}

	event := Envelope{Source: "slack", ID: githubTestID}
	if result := a.admit(event, []byte("{}"), time.Now()); result != "accepted" {
		t.Fatalf("cross-source identity collided: %s", result)
	}

	digest := sha256.Sum256([]byte("identical-body"))
	githubEvent := Envelope{Source: "github", ID: "123e4567-e89b-12d3-a456-426614174100"}
	if result := a.admitGitHub(githubEvent, []byte("{}"), digest, time.Now().Add(11*time.Minute)); result != "accepted" {
		t.Fatalf("expired digest did not permit replay: %s", result)
	}
	restarted := newAdmission()
	if result := restarted.admitGitHub(githubEvent, []byte("{}"), digest, time.Now()); result != "accepted" {
		t.Fatalf("fresh in-memory listener did not permit replay: %s", result)
	}
}

func TestGitHubActiveBodyReadersAreBounded(t *testing.T) {
	a := newAdmission()
	readers := make(chan struct{}, githubReaderLimit)
	release := make(chan struct{})
	handler := githubHandler(githubTestSecret, map[int64]bool{42: true}, a, func(Envelope, string) {}, readers)
	body := githubBody(42, "opened")
	started := make(chan struct{}, githubReaderLimit)
	var wg sync.WaitGroup
	for range githubReaderLimit {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := githubRequest(body, "pull_request", githubTestID, githubTestSecret)
			req.Body = &blockedBody{started: started, release: release, body: []byte(body)}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			if response.Code != http.StatusAccepted && response.Code != http.StatusOK {
				t.Errorf("body reader status=%d", response.Code)
			}
		}()
	}
	for range githubReaderLimit {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("body reader did not enter bounded read slot")
		}
	}
	tooMany := httptest.NewRecorder()
	handler.ServeHTTP(tooMany, githubRequest(body, "pull_request", "123e4567-e89b-12d3-a456-426614174099", githubTestSecret))
	if tooMany.Code != http.StatusServiceUnavailable {
		t.Fatalf("excess body reader status=%d", tooMany.Code)
	}
	close(release)
	wg.Wait()
}

type blockedBody struct {
	started chan<- struct{}
	release <-chan struct{}
	body    []byte
	once    sync.Once
}

func (body *blockedBody) Read(p []byte) (int, error) {
	body.once.Do(func() { body.started <- struct{}{} })
	<-body.release
	if len(body.body) == 0 {
		return 0, io.EOF
	}
	n := copy(p, body.body)
	body.body = body.body[n:]
	return n, nil
}
func (*blockedBody) Close() error { return nil }

func TestGitHubLoopbackSourceServesAndStops(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	var diagnostics capture
	events := make(chan Envelope, 1)
	done := make(chan error, 1)
	go func() {
		done <- Capture(ctx, CaptureOptions{
			Sources: []SourceOptions{{Name: "github", Secret: githubTestSecret, Addr: addr, RepositoryIDs: []int64{42}}},
			Out:     io.Discard, Err: &diagnostics,
			OnEvent: func(event Envelope, _ func(string, string, string), _ io.Writer) { events <- event },
		})
	}()
	endpoint := "http://" + addr + githubPath
	client := &http.Client{Timeout: time.Second}
	var response *http.Response
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(githubBody(42, "opened")))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-GitHub-Delivery", githubTestID)
		request.Header.Set("X-GitHub-Event", "pull_request")
		request.Header.Set("X-Hub-Signature-256", githubHMAC(githubBody(42, "opened"), githubTestSecret))
		response, err = client.Do(request)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err != nil {
		cancel()
		<-done
		t.Fatalf("could not connect to loopback source: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("loopback webhook status=%d", response.StatusCode)
	}
	select {
	case event := <-events:
		if event.Source != "github" || event.ID != githubTestID {
			t.Fatalf("dispatched wrong event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("accepted webhook was not dispatched")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HTTP source did not stop on cancellation")
	}
	if strings.Contains(diagnostics.text(), githubTestSecret) || strings.Contains(diagnostics.text(), "private-body") {
		t.Fatalf("diagnostics leaked request data: %s", diagnostics.text())
	}
}

func TestGitHubReturns202WithoutWaitingForSlowDispatch(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	started := make(chan struct{})
	done := make(chan error, 1)
	var diagnostics capture
	go func() {
		done <- Capture(ctx, CaptureOptions{
			Sources: []SourceOptions{{Name: "github", Secret: githubTestSecret, Addr: addr, RepositoryIDs: []int64{42}}},
			Out:     io.Discard, Err: &diagnostics,
			OnEvent: func(_ Envelope, report func(string, string, string), _ io.Writer) {
				close(started)
				<-release
				report("slow-route", "failed", "provider_failure")
			},
		})
	}()
	defer func() {
		unblock()
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("listener did not stop")
		}
	}()
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(diagnostics.text(), "source=github outcome=connected") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(diagnostics.text(), "source=github outcome=connected") {
		t.Fatal("loopback source did not start")
	}
	endpoint := "http://" + addr + githubPath
	client := &http.Client{Timeout: time.Second}
	request, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(githubBody(42, "opened")))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Delivery", githubTestID)
	request.Header.Set("X-GitHub-Event", "pull_request")
	request.Header.Set("X-Hub-Signature-256", githubHMAC(githubBody(42, "opened"), githubTestSecret))
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("slow-dispatch response=%d", response.StatusCode)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("accepted event was not dispatched")
	}
	unblock()
}

func TestGitHubHTTPHeaderLimit(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var diagnostics capture
	go func() {
		done <- Capture(ctx, CaptureOptions{Sources: []SourceOptions{{Name: "github", Secret: githubTestSecret, Addr: addr, RepositoryIDs: []int64{42}}}, Out: io.Discard, Err: &diagnostics})
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("listener did not stop")
		}
	}()
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(diagnostics.text(), "source=github outcome=connected") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(diagnostics.text(), "source=github outcome=connected") {
		t.Fatal("loopback source did not start")
	}
	connection, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	prefix, suffix := "POST /github HTTP/1.1\r\nHost: localhost\r\nX-Oversized: ", "\r\n\r\n"
	request := prefix + strings.Repeat("x", githubHeaderLimit+1-len(prefix)-len(suffix)) + suffix
	if _, err := io.WriteString(connection, request); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodPost})
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusRequestHeaderFieldsTooLarge || len(body) != 0 {
		t.Fatalf("oversized-header response was not body-free: status=%d body=%q err=%v", response.StatusCode, body, err)
	}
}

func TestGitHubSignedBytesUseExactRawBody(t *testing.T) {
	a := newAdmission()
	handler := githubFixtureHandler(a)
	body := githubBody(42, "opened")
	req := githubRequest(body, "pull_request", githubTestID, githubTestSecret)
	req.Body = io.NopCloser(strings.NewReader(strings.Replace(body, "opened", "closed", 1)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized || len(a.events) != 0 {
		t.Fatalf("tampered signed bytes status=%d queued=%d", response.Code, len(a.events))
	}
}

func TestAdmissionDigestEvictionAndFullBufferDoNotReserve(t *testing.T) {
	a := newAdmission()
	a.capacity = 1
	now := time.Now()
	first := Envelope{Source: "github", ID: githubTestID}
	firstDigest := sha256.Sum256([]byte("first"))
	if result := a.admitGitHub(first, []byte("first"), firstDigest, now); result != "accepted" {
		t.Fatal(result)
	}
	for range ingressLimit - 1 {
		a.events <- []byte("filler")
	}
	second := Envelope{Source: "github", ID: "123e4567-e89b-12d3-a456-426614174001"}
	secondDigest := sha256.Sum256([]byte("second"))
	if result := a.admitGitHub(second, []byte("second"), secondDigest, now); result != "not_sent/busy" || len(a.ids) != 1 || len(a.digests) != 1 {
		t.Fatalf("full buffer reserved identity/digest: result=%s ids=%d digests=%d", result, len(a.ids), len(a.digests))
	}
	<-a.events
	if result := a.admitGitHub(second, []byte("second"), secondDigest, now); result != "accepted" {
		t.Fatalf("evicted oldest entry did not admit new event: %s", result)
	}
	if _, ok := a.digests[firstDigest]; ok || a.digests[secondDigest] == nil {
		t.Fatal("digest cache did not evict in admission order")
	}
}

func TestGitHubStatusResponseHasNoProviderBody(t *testing.T) {
	a := newAdmission()
	req := githubRequest(githubBody(42, "opened"), "pull_request", githubTestID, githubTestSecret)
	response := httptest.NewRecorder()
	githubFixtureHandler(a).ServeHTTP(response, req)
	if response.Code != http.StatusAccepted || response.Body.Len() != 0 || bytes.Contains(response.Body.Bytes(), []byte("private")) {
		t.Fatalf("unsafe status response: %d %q", response.Code, response.Body.String())
	}
}
