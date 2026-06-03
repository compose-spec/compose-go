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

package node

import (
	"testing"

	"go.yaml.in/yaml/v4"
)

// FuzzNormalizeAliases feeds the alias unfolder arbitrary YAML
// documents and checks that the function terminates (the
// defaultMaxAliasNodes cap is the production-grade defense against
// alias bombs; the fuzz target validates that the cap is actually
// honored across the input space).
func FuzzNormalizeAliases(f *testing.F) {
	corpus := []string{
		`x-a: &a {k: v}
services:
  web:
    image: nginx`,
		`x-a: &a
  k: v
x-b: &b
  <<: *a
  z: 1
services:
  s:
    <<: *b`,
		`x-a: &a [1, 2, 3]
x-b: &b [*a, *a]
x-c: [*b, *b]`,
		`x-a: &a {k: v}
x-b: &b [*a, *a, *a]
x-c: &c [*b, *b, *b]
services:
  svc:
    image: alpine`,
		``,
	}
	for _, s := range corpus {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		var n yaml.Node
		if err := yaml.Unmarshal([]byte(src), &n); err != nil {
			t.Skip()
		}
		if n.Kind == 0 {
			t.Skip()
		}
		// NormalizeAliases either returns nil (bounded unfold), an
		// "excessive aliasing" cap hit, or a cycle error -- all three
		// are acceptable terminations. A panic would fail the fuzz
		// harness automatically.
		_ = NormalizeAliases(&n)
	})
}
