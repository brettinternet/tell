package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

type fileValue struct {
	key, text string
	argv      []string
	routes    []Route
	line      int
}

// parse validates the entire layer before applying it, even when higher layers
// override it. As in Noon, decoder errors never echo raw configuration values.
func parse(data []byte) ([]fileValue, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); errors.Is(err, io.EOF) {
		// Empty, whitespace-only, and comment-only files are empty layers.
		return nil, nil
	} else if err != nil {
		return nil, errors.New("invalid YAML: expected one mapping; check syntax")
	}
	if len(document.Content) != 1 {
		return nil, errors.New("expected a YAML mapping")
	}
	var values []fileValue
	if err := parseMapping(document.Content[0], "", &values); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("expected exactly one YAML document")
	}
	return values, nil
}

func parseMapping(node *yaml.Node, prefix string, values *[]fileValue) error {
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" || node.Anchor != "" {
		return fmt.Errorf("line %d: %s must be a mapping without anchors or aliases", node.Line, prefixName(prefix))
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" {
			return fmt.Errorf("line %d: keys must be strings; merge keys and aliases are not supported", key.Line)
		}
		name := prefix + key.Value
		if strings.Contains(key.Value, ".") {
			return fmt.Errorf("line %d: use nested mappings, not dotted field %q", key.Line, name)
		}
		if seen[key.Value] {
			return fmt.Errorf("line %d: duplicate field %q", key.Line, name)
		}
		seen[key.Value] = true
		if value.Anchor != "" || value.Kind == yaml.AliasNode {
			return fmt.Errorf("line %d: anchors and aliases are not supported", value.Line)
		}
		switch name {
		case "version":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" || value.Value != "1" {
				return fmt.Errorf("line %d: version must be 1", value.Line)
			}
		case "listen", "listen.slack":
			*values = append(*values, fileValue{key: name, text: "configured", line: value.Line})
			if err := parseMapping(value, name+".", values); err != nil {
				return err
			}
		case "listen.routes":
			routes, err := parseRoutes(value)
			if err != nil {
				return err
			}
			*values = append(*values, fileValue{key: name, routes: routes, line: value.Line})
		case "notify", "apprise", "herdr", "send", "secrets", "mcp":
			if err := parseMapping(value, name+".", values); err != nil {
				return err
			}
		case "notify.command":
			if value.Kind != yaml.SequenceNode || value.Tag != "!!seq" || len(value.Content) == 0 {
				return fmt.Errorf("line %d: notify.command must be a non-empty argv list", value.Line)
			}
			argv := make([]string, len(value.Content))
			for i, arg := range value.Content {
				if arg.Kind != yaml.ScalarNode || arg.Tag != "!!str" || arg.Anchor != "" || strings.ContainsRune(arg.Value, 0) {
					return fmt.Errorf("line %d: notify.command arguments must be strings without NUL, anchors, or aliases", arg.Line)
				}
				argv[i] = arg.Value
			}
			if strings.TrimSpace(argv[0]) == "" {
				return fmt.Errorf("line %d: notify.command executable must not be empty", value.Line)
			}
			*values = append(*values, fileValue{key: name, argv: argv, line: value.Line})
		case "listen.slack.app_token_env", "secrets.policy", "timeout", "notify.via", "apprise.config", "apprise.tag", "apprise.format", "herdr.session", "send.via", "send.to", "send.from":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("line %d: %s must be a string", value.Line, name)
			}
			*values = append(*values, fileValue{key: name, text: value.Value, line: value.Line})
		case "mcp.agent_targets":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
				return fmt.Errorf("line %d: %s must be a boolean", value.Line, name)
			}
			*values = append(*values, fileValue{key: name, text: value.Value, line: value.Line})
		default:
			return fmt.Errorf("line %d: unknown field %q", key.Line, name)
		}
	}
	return nil
}

func prefixName(prefix string) string {
	if prefix == "" {
		return "configuration"
	}
	return prefix[:len(prefix)-1]
}

var yamlJSONNumber = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)

func parseRoutes(node *yaml.Node) ([]Route, error) {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" || node.Anchor != "" {
		return nil, fmt.Errorf("line %d: listen.routes must be a list", node.Line)
	}
	routes := make([]Route, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Anchor != "" || item.Kind == yaml.AliasNode || item.Kind != yaml.MappingNode || item.Tag != "!!map" {
			return nil, fmt.Errorf("line %d: each listen route must be a mapping without aliases", item.Line)
		}
		route, err := parseRoute(item)
		if err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}
	if err := validateRouteSyntax(routes); err != nil {
		return nil, fmt.Errorf("line %d: %w", node.Line, err)
	}
	return routes, nil
}

func parseRoute(node *yaml.Node) (Route, error) {
	var route Route
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || seen[key.Value] {
			return Route{}, fmt.Errorf("line %d: route keys must be unique strings", key.Line)
		}
		seen[key.Value] = true
		if value.Anchor != "" || value.Kind == yaml.AliasNode {
			return Route{}, fmt.Errorf("line %d: route aliases are not supported", value.Line)
		}
		switch key.Value {
		case "name", "source", "type":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" || strings.ContainsAny(value.Value, "\x00\x1b\r\n") {
				return Route{}, fmt.Errorf("line %d: route %s must be a string without controls", value.Line, key.Value)
			}
			switch key.Value {
			case "name":
				route.Name = value.Value
			case "source":
				route.Source = value.Value
			case "type":
				route.Type = value.Value
			}
		case "match":
			match, err := parseRouteMatch(value)
			if err != nil {
				return Route{}, err
			}
			route.Match = match
		case "run":
			if value.Kind != yaml.SequenceNode || value.Tag != "!!seq" || len(value.Content) == 0 {
				return Route{}, fmt.Errorf("line %d: route run must be a non-empty argv list", value.Line)
			}
			route.Run = make([]string, len(value.Content))
			for i, arg := range value.Content {
				if arg.Kind != yaml.ScalarNode || arg.Tag != "!!str" || arg.Anchor != "" || strings.ContainsRune(arg.Value, 0) {
					return Route{}, fmt.Errorf("line %d: route run arguments must be strings without NUL, anchors, or aliases", arg.Line)
				}
				route.Run[i] = arg.Value
			}
		case "notify":
			action, err := parseNotifyAction(value)
			if err != nil {
				return Route{}, err
			}
			route.Notify = &action
		case "send":
			action, err := parseSendAction(value)
			if err != nil {
				return Route{}, err
			}
			route.Send = &action
		default:
			return Route{}, fmt.Errorf("line %d: unknown route field %q", key.Line, key.Value)
		}
	}
	return route, nil
}

func parseNotifyAction(node *yaml.Node) (NotifyAction, error) {
	var action NotifyAction
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" || node.Anchor != "" {
		return action, fmt.Errorf("line %d: route notify action must be a mapping", node.Line)
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || seen[key.Value] {
			return action, fmt.Errorf("line %d: notify action keys must be unique strings", key.Line)
		}
		seen[key.Value] = true
		if value.Anchor != "" || value.Kind == yaml.AliasNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			return action, fmt.Errorf("line %d: notify action %s must be a string without aliases", value.Line, key.Value)
		}
		if key.Value != "title" && strings.TrimSpace(value.Value) == "" {
			return action, fmt.Errorf("line %d: notify action %s must not be empty", value.Line, key.Value)
		}
		switch key.Value {
		case "message":
			action.Message = value.Value
		case "title":
			action.Title, action.TitleSet = value.Value, true
		case "type":
			action.Type = value.Value
		case "via":
			action.Via = value.Value
		case "herdr_session":
			action.HerdrSession = value.Value
		case "apprise_tag":
			action.AppriseTag = value.Value
		default:
			return action, fmt.Errorf("line %d: unknown notify action field %q", key.Line, key.Value)
		}
	}
	return action, nil
}

func parseSendAction(node *yaml.Node) (SendAction, error) {
	var action SendAction
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" || node.Anchor != "" {
		return action, fmt.Errorf("line %d: route send action must be a mapping", node.Line)
	}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || seen[key.Value] {
			return action, fmt.Errorf("line %d: send action keys must be unique strings", key.Line)
		}
		seen[key.Value] = true
		if value.Anchor != "" || value.Kind == yaml.AliasNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			return action, fmt.Errorf("line %d: send action %s must be a string without aliases", value.Line, key.Value)
		}
		if strings.TrimSpace(value.Value) == "" {
			return action, fmt.Errorf("line %d: send action %s must not be empty", value.Line, key.Value)
		}
		switch key.Value {
		case "message":
			action.Message = value.Value
		case "via":
			action.Via = value.Value
		case "to":
			action.To = value.Value
		case "herdr_session":
			action.HerdrSession = value.Value
		case "intent":
			action.Intent = value.Value
		default:
			return action, fmt.Errorf("line %d: unknown send action field %q", key.Line, key.Value)
		}
	}
	return action, nil
}

func parseRouteMatch(node *yaml.Node) (map[string]Predicate, error) {
	if node.Kind != yaml.MappingNode || node.Tag != "!!map" || node.Anchor != "" {
		return nil, fmt.Errorf("line %d: route match must be a mapping", node.Line)
	}
	match := make(map[string]Predicate, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || strings.ContainsAny(key.Value, "\x00\x1b\r\n") {
			return nil, fmt.Errorf("line %d: predicate path must be a string", key.Line)
		}
		if _, exists := match[key.Value]; exists {
			return nil, fmt.Errorf("line %d: duplicate predicate path", key.Line)
		}
		if value.Anchor != "" || value.Kind == yaml.AliasNode {
			return nil, fmt.Errorf("line %d: predicate aliases are not supported", value.Line)
		}
		predicate, err := parsePredicate(value)
		if err != nil {
			return nil, err
		}
		match[key.Value] = predicate
	}
	return match, nil
}

func parsePredicate(node *yaml.Node) (Predicate, error) {
	if node.Kind == yaml.MappingNode {
		if node.Tag != "!!map" || node.Anchor != "" || len(node.Content) != 2 {
			return Predicate{}, fmt.Errorf("line %d: predicate object must contain only prefix", node.Line)
		}
		key, value := node.Content[0], node.Content[1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value != "prefix" || key.Anchor != "" || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || value.Anchor != "" || strings.ContainsAny(value.Value, "\x00\x1b\r\n") {
			return Predicate{}, fmt.Errorf("line %d: predicate prefix must be a string", node.Line)
		}
		prefix := value.Value
		return Predicate{Prefix: &prefix}, nil
	}
	if node.Kind != yaml.ScalarNode {
		return Predicate{}, fmt.Errorf("line %d: predicate must be a string, number, boolean, or prefix object", node.Line)
	}
	switch node.Tag {
	case "!!str":
		if strings.ContainsAny(node.Value, "\x00\x1b\r\n") {
			return Predicate{}, fmt.Errorf("line %d: predicate string contains controls", node.Line)
		}
		return Predicate{Value: node.Value}, nil
	case "!!bool":
		value, err := strconv.ParseBool(node.Value)
		if err != nil {
			return Predicate{}, fmt.Errorf("line %d: predicate boolean must be true or false", node.Line)
		}
		return Predicate{Value: value}, nil
	case "!!int", "!!float":
		if !yamlJSONNumber.MatchString(node.Value) {
			return Predicate{}, fmt.Errorf("line %d: predicate number must be a finite JSON number", node.Line)
		}
		return Predicate{Value: json.Number(node.Value)}, nil
	default:
		return Predicate{}, fmt.Errorf("line %d: predicate must be a string, number, boolean, or prefix object", node.Line)
	}
}
