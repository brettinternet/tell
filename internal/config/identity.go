package config

import (
	"regexp"
	"strings"
)

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

var (
	e164Handle  = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)
	emailHandle = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)
)

// ValidIMessageHandle accepts one E.164 phone number or one plain email address.
func ValidIMessageHandle(to string) bool {
	return e164Handle.MatchString(to) || emailHandle.MatchString(to) && to[0] != '-'
}
