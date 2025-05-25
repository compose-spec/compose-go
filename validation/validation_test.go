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
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/goccy/go-yaml"
	"gotest.tools/v3/assert"
)

func TestValidateSecret(t *testing.T) {
	checker := checks["configs.*"]
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			name: "file config",
			input: `
name: test
file: ./httpd.conf
`,
			err: "",
		},
		{
			name: "environment config",
			input: `
name: test
environment: CONFIG
`,
			err: "",
		},
		{
			name: "inlined config",
			input: `
name: test
content: foo=bar
`,
			err: "",
		},
		{
			name: "conflict config",
			input: `
name: test
environment: CONFIG
content: foo=bar
`,
			err: "configs.test: file|environment|content attributes are mutually exclusive",
		},
		{
			name: "missing config",
			input: `
name: test
`,
			err: "configs.test: one of file|environment|content must be set",
		},
		{
			name: "external config",
			input: `
name: test
external: true
`,
			err: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input map[string]any
			err := yaml.Unmarshal([]byte(tt.input), &input)
			assert.NilError(t, err)
			err = checker(input, tree.NewPath("configs.test"))
			if tt.err == "" {
				assert.NilError(t, err)
			} else {
				assert.Equal(t, tt.err, err.Error())
			}
		})
	}
}

func TestIPAddress(t *testing.T) {
	checker := checks["services.*.ports.*"]
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			name: "port long syntax, invalid IP",
			input: `
host_ip: notavalidip
target: 1234
published: "1234"
`,
			err: "configs.test.ports[0]: invalid ip address: notavalidip",
		},
		{
			name: "port long syntax, no IP",
			input: `
target: 1234
published: "1234"
`,
		},
		{
			name: "port long syntax, valid IP",
			input: `
host_ip: 192.168.3.4
target: 1234
published: "1234"
`,
		},
	}

	for _, tt := range tests {
		var input map[string]any
		err := yaml.Unmarshal([]byte(tt.input), &input)
		assert.NilError(t, err)
		err = checker(input, tree.NewPath("configs.test.ports[0]"))
		if tt.err == "" {
			assert.NilError(t, err)
		} else {
			assert.Equal(t, tt.err, err.Error())
		}
	}
}
