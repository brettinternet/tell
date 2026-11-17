package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type fileValue struct {
	key, text string
	argv      []string
	routes    []Route
	steps     []NotifyStep
	strings   []string
	repoIDs   []int64
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

type fieldKind int

const (
	fieldUnknown fieldKind = iota
	fieldVersion
	fieldListen // a mapping that also marks its listener as configured
	fieldMapping
	fieldRoutes
	fieldRepositoryIDs
	fieldNotifyVia
	fieldFallbackOn
	fieldArgv
	fieldString
	fieldBool
)

// fields is the file schema: every accepted dotted key and how it parses.
var fields = map[string]fieldKind{
	"version": fieldVersion,
	"listen":  fieldListen, "listen.slack": fieldListen, "listen.github": fieldListen, "listen.http": fieldListen,
	"listen.routes": fieldRoutes, "listen.github.repository_ids": fieldRepositoryIDs,
	"listen.slack.app_token_env": fieldString, "listen.github.secret_env": fieldString, "listen.http.addr": fieldString,
	"notify": fieldMapping, "notify.desktop": fieldMapping, "apprise": fieldMapping, "shoutrrr": fieldMapping, "imessage": fieldMapping,
	"pushover": fieldMapping, "herdr": fieldMapping, "send": fieldMapping, "secrets": fieldMapping, "mcp": fieldMapping,
	"notify.via": fieldNotifyVia, "notify.fallback_on": fieldFallbackOn, "notify.fallback_notice": fieldString,
	"notify.command": fieldArgv, "notify.desktop.on_click": fieldArgv,
	"secrets.policy": fieldString, "timeout": fieldString,
	"apprise.config": fieldString, "apprise.tag": fieldString, "apprise.format": fieldString,
	"shoutrrr.urls_env": fieldString, "imessage.to": fieldString, "herdr.session": fieldString,
	"send.via": fieldString, "send.to": fieldString, "send.from": fieldString,
	"pushover.token_env": fieldString, "pushover.user_env": fieldString, "pushover.priority": fieldString,
	"pushover.retry": fieldString, "pushover.expire": fieldString, "pushover.device": fieldString,
	"mcp.agent_targets": fieldBool,
}

// fieldsUnder lists the key names accepted directly within prefix.
func fieldsUnder(prefix string) []string {
	var names []string
	for name := range fields {
		if rest, ok := strings.CutPrefix(name, prefix); ok && !strings.Contains(rest, ".") {
			names = append(names, rest)
		}
	}
	return names
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
		switch fields[name] {
		case fieldVersion:
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" || value.Value != "1" {
				return fmt.Errorf("line %d: version must be 1", value.Line)
			}
		case fieldListen:
			*values = append(*values, fileValue{key: name, text: "configured", line: value.Line})
			if err := parseMapping(value, name+".", values); err != nil {
				return err
			}
		case fieldRoutes:
			routes, err := parseRoutes(value)
			if err != nil {
				return err
			}
			*values = append(*values, fileValue{key: name, routes: routes, line: value.Line})
		case fieldRepositoryIDs:
			repositoryIDs, err := parseRepositoryIDs(value)
			if err != nil {
				return err
			}
			*values = append(*values, fileValue{key: name, repoIDs: repositoryIDs, line: value.Line})
		case fieldNotifyVia:
			if value.Kind == yaml.SequenceNode {
				steps, err := parseNotifySteps(value)
				if err != nil {
					return err
				}
				*values = append(*values, fileValue{key: name, steps: steps, line: value.Line})
			} else if value.Kind == yaml.ScalarNode && value.Tag == "!!str" {
				*values = append(*values, fileValue{key: name, text: value.Value, line: value.Line})
			} else {
				return fmt.Errorf("line %d: notify.via must be a route string or an ordered route list", value.Line)
			}
		case fieldFallbackOn:
			fallback, err := parseStringList(value, "notify.fallback_on")
			if err != nil {
				return err
			}
			*values = append(*values, fileValue{key: name, strings: fallback, line: value.Line})
		case fieldMapping:
			if err := parseMapping(value, name+".", values); err != nil {
				return err
			}
		case fieldArgv:
			if value.Kind != yaml.SequenceNode || value.Tag != "!!seq" || len(value.Content) == 0 {
				return fmt.Errorf("line %d: %s must be a non-empty argv list", value.Line, name)
			}
			argv := make([]string, len(value.Content))
			for i, arg := range value.Content {
				if arg.Kind != yaml.ScalarNode || arg.Tag != "!!str" || arg.Anchor != "" || strings.ContainsRune(arg.Value, 0) {
					return fmt.Errorf("line %d: %s arguments must be strings without NUL, anchors, or aliases", arg.Line, name)
				}
				argv[i] = arg.Value
			}
			if strings.TrimSpace(argv[0]) == "" {
				return fmt.Errorf("line %d: %s executable must not be empty", value.Line, name)
			}
			*values = append(*values, fileValue{key: name, argv: argv, line: value.Line})
		case fieldString:
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("line %d: %s must be a string", value.Line, name)
			}
			*values = append(*values, fileValue{key: name, text: value.Value, line: value.Line})
		case fieldBool:
			if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
				return fmt.Errorf("line %d: %s must be a boolean", value.Line, name)
			}
			*values = append(*values, fileValue{key: name, text: value.Value, line: value.Line})
		default:
			suggestion := ""
			if closest := Closest(key.Value, fieldsUnder(prefix)...); closest != "" {
				suggestion = fmt.Sprintf("; did you mean %q?", prefix+closest)
			}
			return fmt.Errorf("line %d: unknown field %q%s", key.Line, name, suggestion)
		}
	}
	return nil
}

func parseStringList(node *yaml.Node, field string) ([]string, error) {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" || node.Anchor != "" || len(node.Content) == 0 {
		return nil, fmt.Errorf("line %d: %s must be a non-empty list of strings", node.Line, field)
	}
	values := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" || item.Anchor != "" || strings.TrimSpace(item.Value) == "" || strings.ContainsAny(item.Value, "\x00\x1b\r\n") {
			return nil, fmt.Errorf("line %d: %s entries must be non-empty strings without controls", item.Line, field)
		}
		values = append(values, item.Value)
	}
	return values, nil
}

func parseNotifySteps(node *yaml.Node) ([]NotifyStep, error) {
	if node.Tag != "!!seq" || node.Anchor != "" || len(node.Content) == 0 {
		return nil, fmt.Errorf("line %d: notify.via route list must not be empty", node.Line)
	}
	steps := make([]NotifyStep, 0, len(node.Content))
	for _, item := range node.Content {
		if item.Anchor != "" || item.Kind == yaml.AliasNode {
			return nil, fmt.Errorf("line %d: notify.via steps must not use aliases", item.Line)
		}
		if item.Kind == yaml.ScalarNode && item.Tag == "!!str" {
			step := NotifyStep{Via: item.Value}
			if !validNotifyRoute(step.Via) || strings.ContainsAny(step.Via, "\x00\x1b\r\n") {
				return nil, fmt.Errorf("line %d: notify.via step must name stderr, desktop, alert, apprise, shoutrrr, imessage, pushover, herdr, or command%s", item.Line, DidYouMean(step.Via, notifyRoutes...))
			}
			steps = append(steps, step)
			continue
		}
		if item.Kind != yaml.MappingNode || item.Tag != "!!map" || len(item.Content) != 2 {
			return nil, fmt.Errorf("line %d: notify.via step must be a route name or a single-key route options mapping", item.Line)
		}
		key, options := item.Content[0], item.Content[1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || !validNotifyRoute(key.Value) {
			return nil, fmt.Errorf("line %d: notify.via step has an unknown route%s", key.Line, DidYouMean(key.Value, notifyRoutes...))
		}
		if options.Anchor != "" || options.Kind == yaml.AliasNode || options.Kind != yaml.MappingNode || options.Tag != "!!map" {
			return nil, fmt.Errorf("line %d: notify.via %s options must be a mapping without aliases", options.Line, key.Value)
		}
		step := NotifyStep{Via: key.Value}
		if err := parseNotifyStepOptions(options, &step); err != nil {
			return nil, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

var notifyRoutes = []string{"stderr", "desktop", "alert", "apprise", "shoutrrr", "imessage", "pushover", "herdr", "command"}

func validNotifyRoute(route string) bool { return slices.Contains(notifyRoutes, route) }

func parseNotifyStepOptions(node *yaml.Node, step *NotifyStep) error {
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Anchor != "" || seen[key.Value] {
			return fmt.Errorf("line %d: notify.via step options must be unique strings", key.Line)
		}
		seen[key.Value] = true
		if value.Anchor != "" || value.Kind == yaml.AliasNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || strings.TrimSpace(value.Value) == "" || strings.ContainsAny(value.Value, "\x00\x1b\r\n") {
			return fmt.Errorf("line %d: notify.via step option %s must be a non-empty string without controls", value.Line, key.Value)
		}
		if step.Via == "apprise" {
			switch key.Value {
			case "config":
				if strings.Contains(value.Value, "://") {
					return fmt.Errorf("line %d: apprise step config must be a local path, not a URL", value.Line)
				}
				step.Config = value.Value
			case "tag":
				step.Tag = value.Value
			case "format":
				if value.Value != "text" && value.Value != "markdown" && value.Value != "html" {
					return fmt.Errorf("line %d: apprise step format must be text, markdown, or html%s", value.Line, DidYouMean(value.Value, "text", "markdown", "html"))
				}
				step.Format = value.Value
			case "urls_env":
				if !environmentName.MatchString(value.Value) {
					return fmt.Errorf("line %d: apprise step urls_env must be an environment variable name", value.Line)
				}
				step.UrlsEnv = value.Value
			default:
				return fmt.Errorf("line %d: unknown apprise step option %q%s", key.Line, key.Value, DidYouMean(key.Value, "config", "tag", "format", "urls_env"))
			}
		} else if step.Via == "shoutrrr" && key.Value == "urls_env" {
			if !environmentName.MatchString(value.Value) {
				return fmt.Errorf("line %d: shoutrrr step urls_env must be an environment variable name", value.Line)
			}
			step.UrlsEnv = value.Value
		} else if step.Via == "shoutrrr" {
			return fmt.Errorf("line %d: unknown shoutrrr step option %q%s", key.Line, key.Value, DidYouMean(key.Value, "urls_env"))
		} else if step.Via == "imessage" && key.Value == "to" {
			if !ValidIMessageHandle(value.Value) {
				return fmt.Errorf("line %d: imessage step to must be an E.164 phone number or an email address", value.Line)
			}
			step.To = value.Value
		} else if step.Via == "imessage" {
			return fmt.Errorf("line %d: unknown imessage step option %q%s", key.Line, key.Value, DidYouMean(key.Value, "to"))
		} else if step.Via == "pushover" {
			if err := applyPushoverOption(&step.Pushover, key.Value, value.Value); err != nil {
				return fmt.Errorf("line %d: %w", key.Line, err)
			}
		} else if step.Via == "herdr" && key.Value == "session" {
			step.Session = value.Value
		} else if step.Via == "herdr" {
			return fmt.Errorf("line %d: unknown herdr step option %q%s", key.Line, key.Value, DidYouMean(key.Value, "session"))
		} else if step.Via == "alert" && key.Value == "wait" {
			if wait, err := time.ParseDuration(value.Value); err != nil || wait <= 0 {
				return fmt.Errorf("line %d: alert step wait must be a positive Go duration, such as 2m", value.Line)
			}
			step.Wait = value.Value
		} else if step.Via == "alert" {
			return fmt.Errorf("line %d: unknown alert step option %q%s", key.Line, key.Value, DidYouMean(key.Value, "wait"))
		} else {
			return fmt.Errorf("line %d: route %s does not accept step options", key.Line, step.Via)
		}
	}
	return nil
}

func resolveNotifySteps(steps []NotifyStep, base string) []NotifyStep {
	resolved := append([]NotifyStep(nil), steps...)
	for i := range resolved {
		if resolved[i].Config != "" {
			resolved[i].Config = resolvePath(base, resolved[i].Config)
		}
	}
	return resolved
}

func prefixName(prefix string) string {
	if prefix == "" {
		return "configuration"
	}
	return prefix[:len(prefix)-1]
}

func parseRepositoryIDs(node *yaml.Node) ([]int64, error) {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" || node.Anchor != "" || len(node.Content) == 0 {
		return nil, fmt.Errorf("line %d: listen.github.repository_ids must be a non-empty list of positive integers", node.Line)
	}
	ids := make([]int64, 0, len(node.Content))
	seen := make(map[int64]bool, len(node.Content))
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!int" || item.Anchor != "" || !positiveDecimal.MatchString(item.Value) {
			return nil, fmt.Errorf("line %d: listen.github.repository_ids must contain positive decimal integers", item.Line)
		}
		id, err := strconv.ParseInt(item.Value, 10, 64)
		if err != nil || id <= 0 || seen[id] {
			return nil, fmt.Errorf("line %d: listen.github.repository_ids must contain unique positive integers", item.Line)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

var yamlJSONNumber = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$`)
var positiveDecimal = regexp.MustCompile(`^[0-9]+$`)

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
			return Route{}, fmt.Errorf("line %d: unknown route field %q%s", key.Line, key.Value, DidYouMean(key.Value, "name", "source", "type", "match", "run", "notify", "send"))
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
			return action, fmt.Errorf("line %d: unknown notify action field %q%s", key.Line, key.Value, DidYouMean(key.Value, "message", "title", "type", "via", "herdr_session", "apprise_tag"))
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
			return action, fmt.Errorf("line %d: unknown send action field %q%s", key.Line, key.Value, DidYouMean(key.Value, "message", "via", "to", "herdr_session", "intent"))
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
