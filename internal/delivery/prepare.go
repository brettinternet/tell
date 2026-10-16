package delivery

import (
	"strings"
	"unicode/utf8"
)

const (
	titleLimit       = 256
	bodyLimit        = 16 * 1024
	truncationMarker = "[truncated]"
)

// prepare runs before dispatch. Validate the entire input before truncating so
// even a NUL beyond the output limit prevents delivery.
func prepare(message Message) (Message, string) {
	if strings.ContainsRune(message.Title, 0) || strings.ContainsRune(message.Body, 0) {
		return Message{}, "invalid_message"
	}
	message.Title = normalize(message.Title, titleLimit)
	message.Body = normalize(message.Body, bodyLimit)
	if strings.TrimSpace(message.Body) == "" {
		return Message{}, "empty_message"
	}
	return message, ""
}

// normalize removes ECMA-48 escape/control sequences as well as standalone
// controls. Incomplete sequences are discarded through EOF. Output storage is
// bounded; ranging over text also replaces invalid UTF-8 with U+FFFD.
func normalize(text string, limit int) string {
	const (
		plain = iota
		escape
		intermediate
		csi
		controlString
		stringEscape
	)
	state := plain
	var out strings.Builder
	truncated := false
	for _, r := range text {
		if state == controlString || state == stringEscape {
			switch {
			case r == '\a' || r == '\u009c' || state == stringEscape && r == '\\':
				state = plain
			case r == '\x1b':
				state = stringEscape
			default:
				state = controlString
			}
			continue
		}
		if r == '\x1b' {
			state = escape
			continue
		}
		if state == escape {
			// Ignorable controls do not terminate an ESC introducer.
			if r < 0x20 || r == 0x7f {
				continue
			}
			switch r {
			case '[':
				state = csi
			case ']', 'P', 'X', '^', '_':
				state = controlString
			default:
				state = plain
				if r >= 0x20 && r <= 0x2f {
					state = intermediate
				}
			}
			continue
		}
		if state == intermediate || state == csi {
			if state == intermediate && r >= 0x30 && r <= 0x7e || state == csi && r >= 0x40 && r <= 0x7e {
				state = plain
			}
			continue
		}
		switch r {
		case '\u009b':
			state = csi
			continue
		case '\u0090', '\u0098', '\u009d', '\u009e', '\u009f':
			state = controlString
			continue
		}
		if r < 0x20 && r != '\n' && r != '\t' || r >= 0x7f && r <= 0x9f || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 {
			continue
		}
		if truncated || out.Len()+utf8.RuneLen(r) > limit {
			truncated = true
			continue
		}
		out.WriteRune(r)
	}
	result := out.String()
	if truncated {
		end := min(len(result), limit-len(truncationMarker))
		for end > 0 && !utf8.RuneStart(result[end]) {
			end--
		}
		result = result[:end] + truncationMarker
	}
	return result
}
