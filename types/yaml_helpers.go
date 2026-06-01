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
