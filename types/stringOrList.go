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

package types

import (
	"go.yaml.in/yaml/v4"
)

// StringList is a type for fields that can be a string or list of strings
type StringList []string

func (l *StringList) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value == "" {
			*l = nil
		} else {
			*l = []string{node.Value}
		}
	case yaml.SequenceNode:
		list := make([]string, len(node.Content))
		for i, item := range node.Content {
			if item.Kind != yaml.ScalarNode {
				return NodeErrorf(item, "invalid type for string list")
			}
			list[i] = item.Value
		}
		*l = list
	default:
		return NodeErrorf(node, "invalid node kind %d for string list", node.Kind)
	}
	return nil
}

// StringOrNumberList is a type for fields that can be a list of strings or numbers
type StringOrNumberList []string

func (l *StringOrNumberList) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	switch node.Kind {
	case yaml.ScalarNode:
		*l = []string{node.Value}
	case yaml.SequenceNode:
		list := make([]string, len(node.Content))
		for i, item := range node.Content {
			list[i] = item.Value
		}
		*l = list
	default:
		return NodeErrorf(node, "invalid node kind %d for string list", node.Kind)
	}
	return nil
}
