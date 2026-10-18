package listen

import (
	"bytes"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"

	"github.com/brettinternet/tell/internal/config"
)

var routeNumberPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

// Matches returns all eligible routes in configuration order.
func Matches(routes []config.Route, event Envelope) []config.Route {
	var matched []config.Route
	for _, route := range routes {
		if route.Source != event.Source || route.Type != event.Type || !matchesPredicates(route.Match, event.Data) {
			continue
		}
		matched = append(matched, route)
	}
	return matched
}

func matchesPredicates(predicates map[string]config.Predicate, data map[string]json.RawMessage) bool {
	for path, predicate := range predicates {
		value, ok := pathValue(data, path)
		if !ok || !matchesValue(value, predicate) {
			return false
		}
	}
	return true
}

func pathValue(data map[string]json.RawMessage, path string) (json.RawMessage, bool) {
	parts := strings.Split(path, ".")
	var value json.RawMessage
	for i, part := range parts {
		if i == 0 {
			value = data[part]
		} else {
			var object map[string]json.RawMessage
			if json.Unmarshal(value, &object) != nil || object == nil {
				return nil, false
			}
			value = object[part]
		}
		if value == nil {
			return nil, false
		}
	}
	return value, true
}

func matchesValue(raw json.RawMessage, predicate config.Predicate) bool {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return false
	}
	if predicate.Prefix != nil {
		if trimmed[0] != '"' {
			return false
		}
		var value string
		return json.Unmarshal(trimmed, &value) == nil && strings.HasPrefix(value, *predicate.Prefix)
	}
	switch expected := predicate.Value.(type) {
	case string:
		if trimmed[0] != '"' {
			return false
		}
		var value string
		return json.Unmarshal(trimmed, &value) == nil && value == expected
	case bool:
		if trimmed[0] != 't' && trimmed[0] != 'f' {
			return false
		}
		var value bool
		return json.Unmarshal(trimmed, &value) == nil && value == expected
	case json.Number:
		if trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9') {
			return false
		}
		return equalJSONNumbers(string(trimmed), string(expected))
	default:
		return false
	}
}

func equalJSONNumbers(left, right string) bool {
	leftNegative, leftDigits, leftScale, leftOK := canonicalNumber(left)
	rightNegative, rightDigits, rightScale, rightOK := canonicalNumber(right)
	return leftOK && rightOK && leftNegative == rightNegative && leftDigits == rightDigits && leftScale.Cmp(rightScale) == 0
}

// canonicalNumber represents a JSON decimal as sign, significant digits, and
// an arbitrary-precision base-10 scale, avoiding float64 rounding and overflow.
func canonicalNumber(value string) (bool, string, *big.Int, bool) {
	if !routeNumberPattern.MatchString(value) {
		return false, "", nil, false
	}
	negative := strings.HasPrefix(value, "-")
	if negative {
		value = value[1:]
	}
	mantissa, exponentText, hasExponent := strings.Cut(value, "e")
	if !hasExponent {
		mantissa, exponentText, hasExponent = strings.Cut(mantissa, "E")
	}
	exponent := new(big.Int)
	if hasExponent {
		if _, ok := exponent.SetString(exponentText, 10); !ok {
			return false, "", nil, false
		}
	}
	whole, fraction, hasFraction := strings.Cut(mantissa, ".")
	scale := new(big.Int).Set(exponent)
	if hasFraction {
		scale.Sub(scale, big.NewInt(int64(len(fraction))))
	}
	digits := whole + fraction
	digits = strings.TrimLeft(digits, "0")
	if digits == "" {
		return false, "0", new(big.Int), true
	}
	trimmed := strings.TrimRight(digits, "0")
	scale.Add(scale, big.NewInt(int64(len(digits)-len(trimmed))))
	return negative, trimmed, scale, true
}
