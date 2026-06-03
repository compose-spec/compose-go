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

package override

import (
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/tree"
)

// FuzzMergeNode feeds MergeNode arbitrary pairs of valid YAML documents
// and checks that the function returns within a bounded number of
// steps for every well-formed input and that it never panics. The
// fuzz target is a robustness gate for the merge primitive, not a
// behavioral one -- the corpus only exercises shapes the parser
// accepts.
func FuzzMergeNode(f *testing.F) {
	corpus := []string{
		`services:
  web:
    image: nginx`,
		`services:
  web:
    image: caddy
    restart: always`,
		`services:
  api:
    image: alpine
networks:
  default:
    driver: bridge`,
		`x-anchor: &a
  key: value
services:
  web:
    <<: *a
    image: nginx`,
		`services:
  web:
    ports:
      - 80
      - "443:443"`,
		``,
		`{}`,
	}
	for _, l := range corpus {
		for _, r := range corpus {
			f.Add(l, r)
		}
	}
	f.Fuzz(func(t *testing.T, left, right string) {
		var leftNode, rightNode yaml.Node
		if err := yaml.Unmarshal([]byte(left), &leftNode); err != nil {
			t.Skip()
		}
		if err := yaml.Unmarshal([]byte(right), &rightNode); err != nil {
			t.Skip()
		}
		if leftNode.Kind == 0 || rightNode.Kind == 0 {
			t.Skip()
		}
		// Unwrap the document wrapper so MergeNode sees mapping roots,
		// matching the way the loader invokes it.
		l := &leftNode
		if l.Kind == yaml.DocumentNode && len(l.Content) == 1 {
			l = l.Content[0]
		}
		r := &rightNode
		if r.Kind == yaml.DocumentNode && len(r.Content) == 1 {
			r = r.Content[0]
		}
		if l.Kind != yaml.MappingNode || r.Kind != yaml.MappingNode {
			t.Skip()
		}
		_, _ = MergeNode(l, r, tree.NewPath())
	})
}
