package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

type fileValue struct {
	key, text string
	argv      []string
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
		case "secrets.policy", "timeout", "notify.via", "apprise.config", "apprise.tag", "apprise.format", "herdr.session", "send.via", "send.to", "send.from":
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
