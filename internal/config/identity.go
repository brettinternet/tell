package config

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
