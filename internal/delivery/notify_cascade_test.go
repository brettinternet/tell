package delivery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brettinternet/tell/internal/config"
)

type delayedFailureWriter struct{ delay time.Duration }

func (w delayedFailureWriter) Write(data []byte) (int, error) {
	time.Sleep(w.delay)
	return 0, errors.New("fixture output failure")
}

// Once a fallback-opted failed result may have delivered, exhaustion cannot
// rewrite it as a safe-to-resend not_sent outcome.
func TestNotifyCascadeTimeoutBudgetBeforeFirstStep(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	cfg := config.Config{Timeout: "1s", Notify: config.Notify{Via: "apprise", Steps: []config.NotifyStep{{Via: "apprise"}, {Via: "stderr"}}}}
	result := Notify(ctx, cfg, Message{Body: "body"}, &strings.Builder{})
	if result.Outcome != "not_sent" || result.Route != "apprise" || result.Failure == nil || result.Failure.Reason != "timeout_budget" || len(result.Attempts) != 1 || result.Attempts[0].Reason != "timeout_budget" {
		t.Fatalf("expired budget result: %+v", result)
	}
}

func TestNotifyCascadeTimeoutPreservesUnsafeFailure(t *testing.T) {
	if _, _, reason := PrepareMessage(Message{Body: "warm secret detector"}, "redact"); reason != "" {
		t.Fatalf("prepare warmup: %s", reason)
	}
	cfg := config.Config{
		Timeout: "100ms",
		Notify: config.Notify{
			Via:        "stderr",
			Steps:      []config.NotifyStep{{Via: "stderr"}, {Via: "command"}},
			FallbackOn: []string{"failed"},
			Command:    []string{"/path/that/must/not/run"},
		},
		Secrets: config.Secrets{Policy: "redact"},
	}
	start := time.Now()
	result := Notify(context.Background(), cfg, Message{Title: "title", Body: "body", Type: "info"}, delayedFailureWriter{delay: 150 * time.Millisecond})
	if result.Outcome != "failed" || result.Route != "stderr" || result.Failure == nil || result.Failure.Reason != "io" || len(result.Attempts) != 1 || result.Attempts[0].Reason != "io" {
		t.Fatalf("timeout cascade result: %+v", result)
	}
	if time.Since(start) > 500*time.Millisecond || strings.Contains(result.Failure.Message, "must/not/run") {
		t.Fatalf("timeout result or duration: %+v (%s)", result, time.Since(start))
	}
}
