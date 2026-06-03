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
	"fmt"

	"go.yaml.in/yaml/v4"
)

// StringList is a type for fields that can be a string or list of strings
type StringList []string

// UnmarshalYAML accepts a string or a sequence of strings and stores the
// values in l. Mirrors DecodeMapstructure for yaml.v4 native decoding.
func (l *StringList) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	switch value.Kind {
	case yaml.ScalarNode:
		*l = []string{value.Value}
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*l = list
	default:
		return fmt.Errorf("invalid yaml kind %d for string list", value.Kind)
	}
	return nil
}

// StringOrNumberList is a type for fields that can be a list of strings or numbers
type StringOrNumberList []string

// UnmarshalYAML accepts a string or a sequence of scalar entries (string or
// number, coerced to their stringified form) and stores the values in l.
// Mirrors DecodeMapstructure for yaml.v4 native decoding.
func (l *StringOrNumberList) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	switch value.Kind {
	case yaml.ScalarNode:
		*l = []string{value.Value}
	case yaml.SequenceNode:
		list := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("string-or-number list expects scalar entries")
			}
			list = append(list, item.Value)
		}
		*l = list
	default:
		return fmt.Errorf("invalid yaml kind %d for string-or-number list", value.Kind)
	}
	return nil
}
