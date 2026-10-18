package config

import (
	"errors"
	"strings"
)

// ParseTemplate validates the native-action placeholder grammar and returns
// literal/path parts for the listener's single-pass renderer.
func ParseTemplate(value string) ([]TemplatePart, error) {
	var parts []TemplatePart
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			parts = append(parts, TemplatePart{Text: literal.String()})
			literal.Reset()
		}
	}
	for i := 0; i < len(value); {
		switch {
		case strings.HasPrefix(value[i:], "$${"):
			literal.WriteString("${")
			i += 3
		case strings.HasPrefix(value[i:], "${"):
			end := strings.IndexByte(value[i+2:], '}')
			if end < 0 {
				return nil, errors.New("unterminated placeholder")
			}
			path := value[i+2 : i+2+end]
			if !matchPath.MatchString(path) {
				return nil, errors.New("invalid placeholder path")
			}
			flush()
			parts = append(parts, TemplatePart{Path: path})
			i += 2 + end + 1
		default:
			literal.WriteByte(value[i])
			i++
		}
	}
	flush()
	return parts, nil
}
