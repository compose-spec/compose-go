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
	"strings"

	"go.yaml.in/yaml/v4"
)

// Labels is a mapping type for labels
type Labels map[string]string

func NewLabelsFromMappingWithEquals(mapping MappingWithEquals) Labels {
	labels := Labels{}
	for k, v := range mapping {
		if v != nil {
			labels[k] = *v
		}
	}
	return labels
}

func (l Labels) Add(key, value string) Labels {
	if l == nil {
		l = Labels{}
	}
	l[key] = value
	return l
}

func (l Labels) AsList() []string {
	s := make([]string, len(l))
	i := 0
	for k, v := range l {
		s[i] = fmt.Sprintf("%s=%s", k, v)
		i++
	}
	return s
}

func (l Labels) ToMappingWithEquals() MappingWithEquals {
	mapping := MappingWithEquals{}
	for k, v := range l {
		mapping[k] = &v
	}
	return mapping
}

// UnmarshalYAML accepts a mapping (key -> value) or a list of "key=value"
// entries and stores the result as a Labels map. Mirrors DecodeMapstructure
// for yaml.v4 native decoding. Numeric and boolean scalar values in the
// mapping form are coerced to their stringified representation via the
// underlying scalar Value (yaml.v4 preserves the source representation in
// Node.Value regardless of Tag).
func (l *Labels) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	switch value.Kind {
	case yaml.MappingNode:
		labels := make(Labels, len(value.Content)/2)
		for i := 0; i+1 < len(value.Content); i += 2 {
			labels[value.Content[i].Value] = scalarToString(value.Content[i+1])
		}
		*l = labels
	case yaml.SequenceNode:
		labels := make(Labels, len(value.Content))
		for _, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("labels list entry must be scalar, got kind %d", item.Kind)
			}
			k, v, _ := strings.Cut(item.Value, "=")
			labels[k] = v
		}
		*l = labels
	default:
		return fmt.Errorf("unexpected yaml kind %d for labels", value.Kind)
	}
	return nil
}
