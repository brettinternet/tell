package config

import "strings"

// SlackTokenEnv returns a name, never its credential value.
func (cfg Config) SlackTokenEnv() string {
	if cfg.Listen.Slack == nil {
		return ""
	}
	return cfg.Listen.Slack.AppTokenEnv
}

// ChildEnv strips inbound credentials even from unrelated outbound providers.
func (cfg Config) ChildEnv(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if name != cfg.SlackTokenEnv() {
			result = append(result, entry)
		}
	}
	return result
}
