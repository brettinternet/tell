package delivery

import (
	"bytes"
	"context"
	"encoding/xml"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Desktop presence says whether someone could see a banner. Detection fails
// open: anything unreadable is unknown, and unknown still sends.
const (
	presenceAttended = "attended"
	presenceLocked   = "locked"
	presenceInactive = "inactive_session"
	presenceUnknown  = "unknown"
)

func desktopPresence(ctx context.Context, timeout time.Duration, platform string) string {
	switch platform {
	case "darwin":
		out, ok := presenceProbe(ctx, timeout, "ioreg", "-n", "Root", "-d1", "-a")
		if !ok {
			return presenceUnknown
		}
		return parseConsolePresence(out, os.Getuid())
	case "linux":
		// "auto" would resolve to an SSH caller's own session, so ask for the
		// user's graphical session explicitly.
		out, ok := presenceProbe(ctx, timeout, "loginctl", "show-user", strconv.Itoa(os.Getuid()), "--property=Display")
		if !ok {
			return presenceUnknown
		}
		id := logindProperties(out)["Display"]
		if !logindSessionID.MatchString(id) {
			return presenceUnknown
		}
		out, ok = presenceProbe(ctx, timeout, "loginctl", "show-session", id, "--property=Active", "--property=LockedHint")
		if !ok {
			return presenceUnknown
		}
		return parseLogindPresence(out)
	}
	return presenceUnknown
}

var logindSessionID = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func presenceProbe(ctx context.Context, timeout time.Duration, name string, args ...string) ([]byte, bool) {
	output := &probeOutput{}
	code, failure := process(ctx, timeout, "desktop", name, args, nil, nil, output, nil)
	if failure != nil || code != 0 || output.overflow {
		return nil, false
	}
	return output.buffer.Bytes(), true
}

func logindProperties(out []byte) map[string]string {
	props := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if key, value, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			props[key] = value
		}
	}
	return props
}

// parseLogindPresence reads `loginctl show-session` KEY=VALUE lines.
func parseLogindPresence(out []byte) string {
	props := logindProperties(out)
	switch {
	case props["LockedHint"] == "yes":
		return presenceLocked
	case props["Active"] == "no":
		return presenceInactive
	case props["Active"] == "yes":
		return presenceAttended
	}
	return presenceUnknown
}

// parseConsolePresence reads the `ioreg -n Root -d1 -a` plist: IOConsoleLocked
// and the IOConsoleUsers entry for uid.
func parseConsolePresence(out []byte, uid int) string {
	root, ok := decodePlist(out).(map[string]any)
	if !ok {
		return presenceUnknown
	}
	if locked, _ := root["IOConsoleLocked"].(bool); locked {
		return presenceLocked
	}
	users, _ := root["IOConsoleUsers"].([]any)
	for _, entry := range users {
		user, _ := entry.(map[string]any)
		if id, _ := user["kCGSSessionUserIDKey"].(string); id != strconv.Itoa(uid) {
			continue
		}
		if locked, _ := user["CGSSessionScreenIsLocked"].(bool); locked {
			return presenceLocked
		}
		switch onConsole, ok := user["kCGSSessionOnConsoleKey"].(bool); {
		case !ok:
			return presenceUnknown
		case onConsole:
			return presenceAttended
		default:
			return presenceInactive
		}
	}
	return presenceUnknown
}

// decodePlist returns the root value of an XML property list: dicts become
// map[string]any, arrays []any, booleans bool, and other scalars their text.
// Malformed input returns nil.
func decodePlist(data []byte) any {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local != "plist" {
			value, err := decodePlistValue(decoder, start)
			if err != nil {
				return nil
			}
			return value
		}
	}
}

func decodePlistValue(decoder *xml.Decoder, start xml.StartElement) (any, error) {
	switch start.Name.Local {
	case "true", "false":
		return start.Name.Local == "true", decoder.Skip()
	case "dict", "array":
		dict := map[string]any{}
		var array []any
		key := ""
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			switch token := token.(type) {
			case xml.EndElement:
				if start.Name.Local == "dict" {
					return dict, nil
				}
				return array, nil
			case xml.StartElement:
				value, err := decodePlistValue(decoder, token)
				if err != nil {
					return nil, err
				}
				switch {
				case start.Name.Local == "array":
					array = append(array, value)
				case token.Name.Local == "key":
					key, _ = value.(string)
				default:
					dict[key] = value
				}
			}
		}
	default:
		var text string
		err := decoder.DecodeElement(&text, &start)
		return text, err
	}
}
