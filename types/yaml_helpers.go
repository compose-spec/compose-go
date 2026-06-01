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

import "go.yaml.in/yaml/v4"

// unwrapDocument peels off the DocumentNode wrapper from n when present, so
// custom UnmarshalYAML implementations can be invoked transparently both by
// yaml.Decoder (which forwards the inner node) and by yaml.Unmarshal on a
// top-level value (which forwards the DocumentNode).
func unwrapDocument(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		return n.Content[0]
	}
	return n
}

// scalarToString returns the string representation of a scalar node,
// treating !!null tagged scalars and nil nodes as empty strings. Numeric
// and boolean scalars are returned verbatim because yaml.v4 preserves the
// source representation in Node.Value regardless of Tag, which mirrors the
// fmt.Sprint(e) behavior of the v2 mapstructure helpers.
func scalarToString(n *yaml.Node) string {
	if n == nil || n.Kind != yaml.ScalarNode {
		return ""
	}
	if n.Tag == "!!null" {
		return ""
	}
	return n.Value
}

// scalarToStringPtr returns a *string for a scalar node, distinguishing the
// !!null tag (returns nil) from an empty string (returns a pointer to ""):
// the same distinction MappingWithEquals encodes with `key=` vs `key`.
func scalarToStringPtr(n *yaml.Node) *string {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag == "!!null" {
		return nil
	}
	v := n.Value
	return &v
}
