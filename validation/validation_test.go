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

package validation

import (
	"fmt"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gopkg.in/yaml.v3"
	"gotest.tools/v3/assert"
)

func TestValidateSecret(t *testing.T) {
	treePaths := []tree.Path{"configs.*", "secrets.*"}
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			name: "file",
			input: `
name: test
file: ./httpd.conf
`,
			err: "",
		},
		{
			name: "environment",
			input: `
name: test
environment: CONFIG
`,
			err: "",
		},
		{
			name: "inlined",
			input: `
name: test
content: foo=bar
`,
			err: "",
		},
		{
			name: "conflict",
			input: `
name: test
environment: CONFIG
content: foo=bar
`,
			err: "%s: file|environment|content attributes are mutually exclusive",
		},
		{
			name: "missing",
			input: `
name: test
`,
			err: "%s: one of file|environment|content must be set",
		},
		{
			name: "external",
			input: `
name: test
external: true
`,
			err: "",
		},
	}
	for _, tp := range treePaths {
		checker := checks[tp]

		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s %s", tt.name, tp.Parent()), func(t *testing.T) {
				var input map[string]any
				testTreePath := fmt.Sprintf("%s.test", tp.Parent())

				err := yaml.Unmarshal([]byte(tt.input), &input)
				assert.NilError(t, err)
				err = checker(input, tree.NewPath(testTreePath))
				if tt.err == "" {
					assert.NilError(t, err)
				} else {
					assert.Equal(t, fmt.Sprintf(tt.err, testTreePath), err.Error())
				}
			})
		}
	}

}
