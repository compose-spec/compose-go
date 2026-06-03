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

package interpolation

import (
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/compose-spec/compose-go/v3/template"
)

// FuzzInterpolateNode feeds InterpolateNode arbitrary scalar templates
// with a small fixed environment. The fuzz target verifies that the
// substitution engine terminates and never panics on any input that
// the YAML parser accepts; behavioral correctness of the substitution
// is covered by the unit tests in the template package.
func FuzzInterpolateNode(f *testing.F) {
	corpus := []string{
		`services:
  web:
    image: nginx:${TAG}`,
		`services:
  web:
    image: nginx:${TAG:-1.0}
    command: echo ${CMD:?cmd required}`,
		`services:
  web:
    environment:
      FOO: ${BAR:-fallback}
      DOUBLE: $$LITERAL`,
		`services:
  web:
    image: ${A}-${B}-${C}`,
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
		env := map[string]string{
			"TAG": "2.0",
			"BAR": "bar",
			"A":   "alpha",
			"B":   "beta",
			"C":   "gamma",
		}
		_ = InterpolateNode(&n, NodeOptions{
			Substitute: template.Substitute,
			LookupValue: func(key string) (string, bool) {
				v, ok := env[key]
				return v, ok
			},
		})
	})
}
