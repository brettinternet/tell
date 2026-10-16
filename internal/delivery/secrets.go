package delivery

import (
	"sort"
	"strings"
	"sync"

	"github.com/spf13/viper"
	leaksconfig "github.com/zricethezav/gitleaks/v8/config"
	"github.com/zricethezav/gitleaks/v8/detect"
)

// Load only the embedded rules, never environment, repository files, or user
// allowlists. Cache configuration, not a detector's mutable per-scan state.
var secretRules = sync.OnceValues(func() (leaksconfig.Config, error) {
	v := viper.New()
	v.SetConfigType("toml")
	if err := v.ReadConfig(strings.NewReader(leaksconfig.DefaultConfig)); err != nil {
		return leaksconfig.Config{}, err
	}
	var cfg leaksconfig.ViperConfig
	if err := v.Unmarshal(&cfg); err != nil {
		return leaksconfig.Config{}, err
	}
	return cfg.Translate()
})

func scanSecrets(message Message, policy string) (Message, int, string) {
	cfg, err := secretRules()
	if err != nil {
		return Message{}, 0, "secret_scan_failed"
	}
	detector := detect.NewDetector(cfg)
	// Message authors cannot opt out of scanning with an inline comment.
	detector.IgnoreGitleaksAllow = true
	count := 0
	for _, field := range []*string{&message.Title, &message.Body} {
		findings := detector.DetectString(*field)
		if len(findings) > 0 && policy == "reject" {
			return Message{}, 0, "secret_detected"
		}
		// Locate every occurrence and merge overlapping ranges before replacing.
		// This avoids partial redaction when different rules overlap or repeat.
		type span struct{ start, end int }
		var spans []span
		seen := make(map[string]bool)
		for _, finding := range findings {
			secret := finding.Secret
			if secret == "" {
				secret = finding.Match
			}
			if secret == "" {
				return Message{}, 0, "secret_scan_failed"
			}
			if seen[secret] {
				continue
			}
			seen[secret] = true
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
