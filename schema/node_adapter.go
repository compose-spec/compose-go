/*
   Copyright 2020 The Compose Specification Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package schema

import (
	"errors"
	"fmt"
	"strconv"

	"go.yaml.in/yaml/v4"
)

// ValidateNode validates a yaml.Node tree against the Compose JSON Schema.
// It is the yaml.Node based counterpart of Validate, intended for the v3
// loader pipeline. The node is converted to its generic Go representation
// (the same one json.Unmarshal would produce) and forwarded to Validate.
//
// The conversion is kept local to this file so the main loader pipeline
// never needs to produce a map[string]any of its own.
func ValidateNode(node *yaml.Node) error {
	if node == nil {
		return errors.New("nil node")
	}
	v, err := nodeToInterface(unwrapDocument(node))
	if err != nil {
		return err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return fmt.Errorf("top-level node must be a mapping, got %T", v)
	}
	return Validate(m)
}

func unwrapDocument(node *yaml.Node) *yaml.Node {
	if node != nil && node.Kind == yaml.DocumentNode && len(node.Content) == 1 {
		return node.Content[0]
	}
	return node
}

// nodeToInterface converts a yaml.Node tree into the same untyped Go
// representation that json.Unmarshal into interface{} would produce:
// mappings become map[string]any, sequences become []any, scalars become
// typed primitives (string, int, float64, bool, nil).
func nodeToInterface(node *yaml.Node) (any, error) {
	if node == nil {
		return nil, nil
	}
	switch node.Kind {
	case yaml.DocumentNode:
		if len(node.Content) == 0 {
			return nil, nil
		}
		return nodeToInterface(node.Content[0])
	case yaml.MappingNode:
		m := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val, err := nodeToInterface(node.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[key] = val
		}
		return m, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(node.Content))
		for _, c := range node.Content {
			v, err := nodeToInterface(c)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.ScalarNode:
		return scalarValue(node), nil
	case yaml.AliasNode:
		if node.Alias != nil {
			return nodeToInterface(node.Alias)
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unsupported yaml kind %d", node.Kind)
}

// scalarValue turns a ScalarNode into a typed Go value based on its yaml tag.
func scalarValue(node *yaml.Node) any {
	switch node.Tag {
	case "!!null":
		return nil
	case "!!bool":
		switch node.Value {
		case "true", "True", "TRUE":
			return true
		case "false", "False", "FALSE":
			return false
		}
		return node.Value
	case "!!int":
		if n, err := strconv.ParseInt(node.Value, 0, 64); err == nil {
			return n
		}
		return node.Value
	case "!!float":
		if f, err := strconv.ParseFloat(node.Value, 64); err == nil {
			return f
		}
		return node.Value
	default:
		return node.Value
	}
}
