package delivery

import (
	"fmt"
	"strings"
)

// Provider output is untrusted and may contain credentials, URLs, or the body.
// Extract only fixed hints; never include a matched line or substring in results.
// Hints are evidence from logs, not authoritative delivery outcomes.
func appriseDiagnostic(code int, output *probeOutput) string {
	message := fmt.Sprintf(" (Apprise exit %d)", code)
	text := strings.ToLower(output.buffer.String())
	for _, hint := range []struct {
		matches []string
		message string
	}{
		{[]string{"unauthorized", "forbidden", "authentication failed", "invalid token"}, "authentication or permission error; check credentials and destination permissions in Apprise"},
		{[]string{"too many requests", "rate limit"}, "rate limiting; check the provider's retry guidance before resending"},
		{[]string{"connection refused", "failed to establish a new connection", "name resolution", "name or service not known"}, "connection error; check network access and the provider endpoint"},
		{[]string{"certificate verify failed", "certificate_verify_failed"}, "TLS certificate error; check the provider certificate and local trust store"},
		{[]string{"timed out", "timeout"}, "timeout; check provider availability and whether delivery occurred"},
		{[]string{"no service(s) to notify", "no services to notify", "no service(s) found", "no services found"}, "no services found; check Apprise configuration and tag selection"},
	} {
		for _, match := range hint.matches {
			if strings.Contains(text, match) {
				message += "; diagnostic hint: " + hint.message
				break
			}
		}
	}
	if output.overflow {
		message += "; diagnostic capture truncated"
	}
	return message + "; run tell doctor --via apprise with the same config/tag settings. For details, run apprise -vv directly with the same settings and a harmless test message (sends again; logs may contain secrets). Check delivery before retrying"
}
