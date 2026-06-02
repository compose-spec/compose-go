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

// Options is a mapping type for options we pass as-is to container runtime
type Options map[string]string

// MultiOptions allow option to be repeated
type MultiOptions map[string][]string

// UnmarshalYAML accepts a mapping of single-valued string options and
// stores it in d. Mirrors DecodeMapstructure for yaml.v4 native decoding.
func (d *Options) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping for options, got kind %d", value.Kind)
	}
	m := make(Options, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		m[value.Content[i].Value] = scalarToString(value.Content[i+1])
	}
	*d = m
	return nil
}

// UnmarshalYAML accepts a mapping where each value is either a scalar or a
// sequence of scalars, and stores the result in d as a slice per key.
func (d *MultiOptions) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping for options, got kind %d", value.Kind)
	}
	m := make(MultiOptions, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch val.Kind {
		case yaml.ScalarNode:
			m[key] = []string{scalarToString(val)}
		case yaml.SequenceNode:
			values := make([]string, 0, len(val.Content))
			for _, item := range val.Content {
				values = append(values, scalarToString(item))
			}
			m[key] = values
		default:
			return fmt.Errorf("option %s: expected scalar or sequence, got kind %d", key, val.Kind)
		}
	}
	*d = m
	return nil
}
