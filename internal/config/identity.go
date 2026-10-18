package config

import "strings"

// ValidIdentity excludes CLI options, whitespace, control characters and selectors.
func ValidIdentity(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] == '-' {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// ValidRecipient is the shared exact-recipient rule for config validation and
// outbound delivery. It deliberately refuses selectors and remote hcom targets.
func ValidRecipient(route, to string) bool {
	if ValidIdentity(to) && !strings.HasSuffix(to, "-") && to != "help" && to != "self" {
		return true
	}
	if route == "herdr" {
		workspace, pane, ok := strings.Cut(to, ":")
		return ok && strings.HasPrefix(workspace, "w") && strings.HasPrefix(pane, "p") && ValidIdentity(workspace) && ValidIdentity(pane)
	}
	return false
}
