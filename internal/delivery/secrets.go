package delivery

import (
	"context"
	"sort"
	"strings"
	"sync"

	leaksconfig "github.com/betterleaks/betterleaks/config"
	"github.com/betterleaks/betterleaks/detect"
)

// Load only the embedded rules, never environment, repository files, or user
// allowlists. Cache configuration, not a detector's mutable per-scan state.
var secretRules = sync.OnceValues(leaksconfig.Default)

// Building a detector writes to its config, so scans sharing the cached config
// must not overlap.
var secretScan sync.Mutex

// RedactDiagnostic removes detected credentials from user input echoed in errors.
// Unlike message preparation, it neither truncates nor changes diagnostic wording.
func RedactDiagnostic(text string) string {
	message, _, reason := scanSecrets(Message{Body: text}, "redact")
	if reason != "" {
		return "[omitted]"
	}
	return message.Body
}

func scanSecrets(message Message, policy string) (Message, int, string) {
	cfg, err := secretRules()
	if err != nil {
		return Message{}, 0, "secret_scan_failed"
	}
	secretScan.Lock()
	defer secretScan.Unlock()
	// Zero ValidationOptions keep scanning offline: no live secret validation.
	detector := detect.NewDetectorContext(context.Background(), cfg, detect.ValidationOptions{})
	// Message authors cannot opt out of scanning with an inline comment
	// (betterleaks:allow or gitleaks:allow).
	detector.IgnoreGitleaksAllow = true
	fields := []*string{&message.Title, &message.Body, &message.URL}
	// Collect secrets from every field before redacting any: a context rule
	// (such as api_key=...) may match in one field while the bare value repeats
	// in the other.
	var secrets []string
	seen := make(map[string]bool)
	for _, field := range fields {
		for _, finding := range detector.DetectString(*field) {
			if policy == "reject" {
				return Message{}, 0, "secret_detected"
			}
			secret := finding.Secret
			if secret == "" {
				secret = finding.Match
			}
			if seen[secret] {
				continue
			}
			// A finding that cannot be located cannot be redacted.
			if secret == "" || !strings.Contains(*field, secret) {
				return Message{}, 0, "secret_scan_failed"
			}
			seen[secret] = true
			secrets = append(secrets, secret)
		}
	}
	count := 0
	for _, field := range fields {
		// Locate every occurrence and merge overlapping ranges before replacing.
		// This avoids partial redaction when different rules overlap or repeat.
		type span struct{ start, end int }
		var spans []span
		for _, secret := range secrets {
			for offset := 0; offset < len(*field); {
				index := strings.Index((*field)[offset:], secret)
				if index < 0 {
					break
				}
				start := offset + index
				spans = append(spans, span{start, start + len(secret)})
				offset = start + 1
			}
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
		var merged []span
		for _, s := range spans {
			if len(merged) > 0 && s.start < merged[len(merged)-1].end {
				merged[len(merged)-1].end = max(merged[len(merged)-1].end, s.end)
			} else {
				merged = append(merged, s)
			}
		}
		var out strings.Builder
		end := 0
		for _, s := range merged {
			out.WriteString((*field)[end:s.start])
			out.WriteString("[REDACTED]")
			end = s.end
		}
		out.WriteString((*field)[end:])
		*field = out.String()
		count += len(merged)
	}
	return message, count, ""
}
