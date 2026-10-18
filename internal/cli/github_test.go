package cli

import (
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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

type lockedBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (buffer *lockedBuffer) Write(data []byte) (int, error) {
	buffer.Lock()
	defer buffer.Unlock()
	return buffer.Buffer.Write(data)
}
func (buffer *lockedBuffer) text() string {
	buffer.Lock()
	defer buffer.Unlock()
	return buffer.Buffer.String()
}

// The subprocess invoked by TestGitHubWebhookDispatchesOnlyMatchingEvents writes
// its received envelope and fixed Tell environment to a private fixture file.
func TestGitHubActionFixtureProcess(t *testing.T) {
	if len(os.Args) == 0 || os.Args[len(os.Args)-1] != "--fixture-github-action" {
		return
	}
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		t.Fatal(err)
	}
	result, err := json.Marshal(map[string]any{
		"envelope": json.RawMessage(body),
		"source":   os.Getenv("TELL_EVENT_SOURCE"),
		"type":     os.Getenv("TELL_EVENT_TYPE"),
		"id":       os.Getenv("TELL_EVENT_ID"),
		"route":    os.Getenv("TELL_ROUTE"),
		"project":  os.Getenv("TELL_PROJECT"),
		"secret":   os.Getenv("TELL_GITHUB_SECRET"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("TELL_GITHUB_CAPTURE_FILE"), result, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestGitHubWebhookDispatchesOnlyMatchingEvents(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	captureFile := filepath.Join(t.TempDir(), "captured-event.json")
	t.Setenv("TELL_GITHUB_SECRET", "fixture-private-webhook-secret")
	t.Setenv("TELL_GITHUB_CAPTURE_FILE", captureFile)
	binary := strconv.Quote(os.Args[0])
	file := filepath.Join(t.TempDir(), "github.yaml")
	configuration := fmt.Sprintf(`listen:
  github:
    secret_env: TELL_GITHUB_SECRET
    repository_ids: [42]
  http:
    addr: %s
  routes:
    - name: opened-pr
      source: github
      type: pull_request
      match: {action: opened}
      run: [%s, %s, --, --fixture-github-action]
`, strconv.Quote(addr), binary, strconv.Quote("-test.run=^TestGitHubActionFixtureProcess$"))
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, diag := &lockedBuffer{}, &lockedBuffer{}
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, []string{"tell", "--config", file, "listen", "github"}, config.BuildOpts{}, out, diag)
	}()
	post := func(deliveryID, eventType, action string) int {
		t.Helper()
		body := fmt.Sprintf(`{"event":%q,"action":%q,"repository":{"id":42,"name":"fixture"},"pull_request":{"title":"fixture-title"}}`, eventType, action)
		client := &http.Client{Timeout: time.Second}
		deadline := time.Now().Add(2 * time.Second)
		for {
			request, err := http.NewRequest(http.MethodPost, "http://"+addr+"/github", strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-GitHub-Delivery", deliveryID)
			request.Header.Set("X-GitHub-Event", eventType)
			request.Header.Set("X-Hub-Signature-256", testHMAC(body, "fixture-private-webhook-secret"))
			response, err := client.Do(request)
			if err == nil {
				defer response.Body.Close()
				return response.StatusCode
			}
			if time.Now().After(deadline) {
				t.Fatal(err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if status := post("123e4567-e89b-12d3-a456-426614174001", "pull_request", "opened"); status != http.StatusAccepted {
		cancel()
		t.Fatalf("matching webhook response=%d", status)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(captureFile); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("matching event did not reach its local command")
		}
		time.Sleep(5 * time.Millisecond)
	}
	captured, err := os.ReadFile(captureFile)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Envelope struct {
			SchemaVersion int                        `json:"schema_version"`
			Source        string                     `json:"source"`
			Type          string                     `json:"type"`
			ID            string                     `json:"id"`
			Data          map[string]json.RawMessage `json:"data"`
		} `json:"envelope"`
		Source, Type, ID, Route, Project, Secret string
	}
	if err := json.Unmarshal(captured, &result); err != nil {
		t.Fatal(err)
	}
	if result.Envelope.SchemaVersion != 1 || result.Envelope.Source != "github" || result.Envelope.Type != "pull_request" || result.Envelope.ID != "123e4567-e89b-12d3-a456-426614174001" || result.Source != "github" || result.Type != "pull_request" || result.ID != result.Envelope.ID || result.Route != "opened-pr" || result.Project == "" || result.Secret != "" {
		t.Fatalf("local action received wrong envelope/environment: %+v", result)
	}
	waitForNoMatch := func(id, eventType string) {
		t.Helper()
		want := fmt.Sprintf(`id=%q type=%q route="" outcome=no_match`, id, eventType)
		deadline := time.Now().Add(2 * time.Second)
		for !strings.Contains(diag.text(), want) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !strings.Contains(diag.text(), want) {
			t.Fatalf("unmatched event did not produce its body-free receipt: %s", diag.text())
		}
	}
	secondID := "123e4567-e89b-12d3-a456-426614174002"
	if status := post(secondID, "pull_request", "closed"); status != http.StatusAccepted {
		cancel()
		t.Fatalf("unmatched valid webhook response=%d", status)
	}
	waitForNoMatch(secondID, "pull_request")
	thirdID := "123e4567-e89b-12d3-a456-426614174003"
	if status := post(thirdID, "issues", "opened"); status != http.StatusAccepted {
		cancel()
		t.Fatalf("unrelated event response=%d", status)
	}
	waitForNoMatch(thirdID, "issues")
	unchanged, err := os.ReadFile(captureFile)
	if err != nil || !bytes.Equal(unchanged, captured) {
		t.Fatalf("unmatched event executed the route: err=%v", err)
	}
	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("listener exit code=%d stderr=%s", code, diag.text())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("listener did not stop")
	}
	if strings.Contains(out.text()+diag.text(), "fixture-private-webhook-secret") || strings.Contains(diag.text(), "fixture-title") {
		t.Fatal("diagnostics leaked secret or body")
	}
}

func testHMAC(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitHubPrintExportsVerifiedEnvelopeWithoutRunningRoutes(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	_ = listener.Close()
	t.Setenv("TELL_GITHUB_SECRET", "private-webhook-secret")
	actionPath, touchErr := exec.LookPath("touch")
	if touchErr != nil {
		t.Fatal(touchErr)
	}
	marker := filepath.Join(t.TempDir(), "action-ran")
	file := filepath.Join(t.TempDir(), "github.yaml")
	configuration := fmt.Sprintf(`listen:
  github:
    secret_env: TELL_GITHUB_SECRET
    repository_ids: [42]
  http: {addr: %q}
  routes:
    - name: opened-pr
      source: github
      type: pull_request
      run: [%q, %q]
`, addr, actionPath, marker)
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var out, diag lockedBuffer
	done := make(chan int, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- Run(ctx, []string{"tell", "--config", file, "listen", "--print", "github"}, config.BuildOpts{}, &out, &diag)
	}()
	defer func() {
		cancel()
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
			t.Error("print listener did not stop")
		}
	}()
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(diag.text(), "source=github outcome=connected") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(diag.text(), "source=github outcome=connected") {
		t.Fatal("print listener did not start")
	}
	body := `{"action":"opened","repository":{"id":42,"name":"private-repository"},"pull_request":{"title":"private-title"}}`
	request, err := http.NewRequest(http.MethodPost, "http://"+addr+"/github", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-GitHub-Delivery", "123e4567-e89b-12d3-a456-426614174009")
	request.Header.Set("X-GitHub-Event", "pull_request")
	request.Header.Set("X-Hub-Signature-256", testHMAC(body, "private-webhook-secret"))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("verified print webhook status=%d", response.StatusCode)
	}
	deadline = time.Now().Add(time.Second)
	for !strings.Contains(out.text(), "private-title") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out.text()), &envelope); err != nil {
		t.Fatalf("printed envelope is not NDJSON: %s %v", out.text(), err)
	}
	if string(envelope["source"]) != `"github"` || string(envelope["id"]) != `"123e4567-e89b-12d3-a456-426614174009"` || !strings.Contains(string(envelope["data"]), "private-title") {
		t.Fatalf("printed envelope lost verified metadata or body: %s", out.text())
	}
	cancel()
	if code := <-done; code != 0 {
		t.Fatalf("print listener shutdown exit=%d stderr=%s", code, diag.text())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("print capture ran its route: %v", err)
	}
	if strings.Contains(diag.text(), "private-webhook-secret") || strings.Contains(diag.text(), "private-title") {
		t.Fatal("print diagnostic leaked credentials or event body")
	}
}

func TestGitHubInspectionAndDoctorNeverBind(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "github.yaml")
	if err := os.WriteFile(file, []byte(fmt.Sprintf("listen:\n  github:\n    secret_env: TELL_GITHUB_SECRET\n    repository_ids: [42]\n  http:\n    addr: %q\n", addr)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELL_GITHUB_SECRET", "private-webhook-secret")
	for _, args := range [][]string{
		{}, {"listen"}, {"--json", "listen"}, {"config"}, {"--json", "config"}, {"config", "check"}, {"doctor"}, {"--json", "doctor"},
	} {
		var out, diag bytes.Buffer
		if code := Run(context.Background(), append([]string{"tell", "--config", file}, args...), config.BuildOpts{}, &out, &diag); code != 0 {
			t.Errorf("%v failed without a listener selection: %d %s", args, code, diag.String())
		}
		if strings.Contains(out.String()+diag.String(), "private-webhook-secret") {
			t.Fatalf("inspection exposed webhook secret for %v", args)
		}
	}
	t.Setenv("TELL_GITHUB_SECRET", "")
	var doctorOut, doctorErr bytes.Buffer
	if code := Run(context.Background(), []string{"tell", "--config", file, "doctor"}, config.BuildOpts{}, &doctorOut, &doctorErr); code != 0 || !strings.Contains(doctorOut.String(), "listen/github: not ready") {
		t.Fatalf("missing inbound secret changed doctor readiness: %d %s %s", code, doctorOut.String(), doctorErr.String())
	}
	probe, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("inspection unexpectedly bound GitHub listener: %v", err)
	}
	_ = probe.Close()
	var out, diag bytes.Buffer
	if code := Run(context.Background(), []string{"tell", "listen", "--help"}, config.BuildOpts{}, &out, &diag); code != 0 || !strings.Contains(out.String(), "github") {
		t.Fatalf("help did not advertise GitHub source: %d %s", code, out.String())
	}
	completion := listenCommand()
	var suggestions bytes.Buffer
	completion.Writer = &suggestions
	completion.ShellComplete(context.Background(), completion)
	if strings.TrimSpace(suggestions.String()) != "slack github" {
		t.Fatalf("source completion omitted supported names: %q", suggestions.String())
	}
}

func TestGitHubSelectedSecretAndBindFailuresHappenBeforeCapture(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	defer listener.Close()
	file := filepath.Join(t.TempDir(), "github.yaml")
	configuration := fmt.Sprintf(`listen:
  github:
    secret_env: TELL_GITHUB_SECRET
    repository_ids: [42]
  http: {addr: %q}
  routes:
    - name: opened-pr
      source: github
      type: pull_request
      run: [/bin/true]
`, addr)
	if err := os.WriteFile(file, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TELL_GITHUB_SECRET", "")
	var out, diag bytes.Buffer
	if code := Run(context.Background(), []string{"tell", "--config", file, "listen", "github"}, config.BuildOpts{}, &out, &diag); code != 1 || !strings.Contains(diag.String(), "selected GitHub credential is missing") {
		t.Fatalf("missing selected secret result=%d stderr=%s", code, diag.String())
	}
	if _, err := net.DialTimeout("tcp", addr, 20*time.Millisecond); err != nil {
		t.Fatalf("missing selected secret attempted to bind: %v", err)
	}
	t.Setenv("TELL_GITHUB_SECRET", "do-not-print-webhook-secret")
	out.Reset()
	diag.Reset()
	if code := Run(context.Background(), []string{"tell", "--config", file, "listen", "github"}, config.BuildOpts{}, &out, &diag); code != 1 {
		t.Fatalf("bind conflict exit=%d stderr=%s", code, diag.String())
	}
	if !strings.Contains(diag.String(), "reason=bind_failed") || strings.Contains(diag.String(), "reason=source_failed") {
		t.Fatalf("bind conflict receipt was missing or misleading: %s", diag.String())
	}
	if strings.Contains(out.String()+diag.String(), "do-not-print-webhook-secret") {
		t.Fatal("bind failure leaked selected webhook secret")
	}
}
