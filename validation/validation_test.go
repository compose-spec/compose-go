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
	"math"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"go.yaml.in/yaml/v4"
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
		t.Run(tt.name, func(t *testing.T) {
			var input map[string]any
			err := yaml.Unmarshal([]byte(tt.input), &input)
			assert.NilError(t, err)
			err = checker(input, tree.NewPath("configs.test.ports[0]"))
			if tt.err == "" {
				assert.NilError(t, err)
			} else {
				assert.Equal(t, tt.err, err.Error())
			}
		})
	}
}

// mem_swappiness is a percentage: the specification restricts it to 0 to 100,
// and the type that models it would accept any amount, in any byte unit. -1, the
// Docker CLI default read by the engine as unset, is accepted as it always was.
func TestValidateMemSwappiness(t *testing.T) {
	checker := checks["services.*.mem_swappiness"]
	tests := []struct {
		name  string
		value any
		err   string
	}{
		{name: "turned off", value: 0},
		{name: "a percentage", value: 60},
		{name: "all pages swappable", value: 100},
		{name: "a numeric string", value: "60"},
		{name: "the default of the Docker CLI", value: -1},
		{name: "the default of the Docker CLI, as a string", value: "-1"},
		{name: "an integer written as a float", value: 1e1},
		{name: "unset", value: nil},
		{
			name:  "above 100",
			value: 101,
			err:   "services.foo.mem_swappiness: invalid value 101: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "below -1",
			value: -2,
			err:   "services.foo.mem_swappiness: invalid value -2: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "a byte value",
			value: "1g",
			err:   "services.foo.mem_swappiness: invalid value 1g: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "text",
			value: "abc",
			err:   "services.foo.mem_swappiness: invalid value abc: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "a fraction",
			value: 0.5,
			err:   "services.foo.mem_swappiness: invalid value 0.5: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "a float beyond 100",
			value: 1e3,
			err:   "services.foo.mem_swappiness: invalid value 1000: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "an integer beyond 64 bits",
			value: uint64(math.MaxInt64) + 1,
			err:   "services.foo.mem_swappiness: invalid value 9223372036854775808: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "a boolean",
			value: true,
			err:   "services.foo.mem_swappiness: invalid value true: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker(tt.value, tree.NewPath("services.foo.mem_swappiness"))
			if tt.err == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tt.err)
			}
		})
	}
}
