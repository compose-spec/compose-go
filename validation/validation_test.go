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
		// a pre-parsed model may hold numbers of any Go type
		{name: "a small integer type", value: int32(60)},
		{name: "an unsigned integer", value: uint8(60)},
		{name: "a float32", value: float32(60)},
		{name: "a named integer type", value: percent(60)},
		{
			name:  "a small integer type, above 100",
			value: int32(101),
			err:   "services.foo.mem_swappiness: invalid value 101: mem_swappiness must be a percentage, an integer between 0 and 100",
		},
		{
			name:  "a named integer type, below -1",
			value: percent(-2),
			err:   "services.foo.mem_swappiness: invalid value -2: mem_swappiness must be a percentage, an integer between 0 and 100",
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

// percent stands for a named number type, such as types.UnitBytes, in a model
// that was parsed before it reaches validation.
type percent int64

// A memory limit, memory reservation or shared memory size is not negative,
// whatever the Go type of the number in a pre-parsed model. Other values are
// for the byte value parser to judge.
func TestValidateNotNegative(t *testing.T) {
	checker := checks["services.*.mem_limit"]
	tests := []struct {
		name  string
		value any
		err   string
	}{
		{name: "a number of bytes", value: 1024},
		{name: "zero", value: 0},
		{name: "a value with a unit", value: "512m"},
		{name: "a numeric string", value: "1024"},
		{name: "unset", value: nil},
		{name: "an unsigned integer", value: uint64(1024)},
		{name: "text is left to the parser", value: "abc"},
		{name: "a negative amount with a unit is left to the parser", value: "-1g"},
		{name: "a negative number", value: -1, err: "services.foo.mem_limit: invalid value -1: must not be negative"},
		{name: "a negative int64", value: int64(-1), err: "services.foo.mem_limit: invalid value -1: must not be negative"},
		{name: "a negative int32", value: int32(-1), err: "services.foo.mem_limit: invalid value -1: must not be negative"},
		{name: "a negative float64", value: float64(-1), err: "services.foo.mem_limit: invalid value -1: must not be negative"},
		{name: "a negative float32", value: float32(-1), err: "services.foo.mem_limit: invalid value -1: must not be negative"},
		{name: "a negative named type", value: percent(-1), err: "services.foo.mem_limit: invalid value -1: must not be negative"},
		{name: "a negative numeric string", value: "-1", err: "services.foo.mem_limit: invalid value -1: must not be negative"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker(tt.value, tree.NewPath("services.foo.mem_limit"))
			if tt.err == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, tt.err)
			}
		})
	}
}

// `external` is a boolean. Interpolation can be skipped, which leaves a
// reference such as `${EXTERNAL:-true}` in its place: the validation checks
// what it can tell, and leaves a value that is not a boolean to the decoding of
// the model, instead of crashing.
// https://github.com/compose-spec/compose-spec/blob/main/07-volumes.md#external
func TestValidateExternalThatIsNotABoolean(t *testing.T) {
	checker := checks["volumes.*"]
	for name, input := range map[string]string{
		"unresolved reference": "external: ${EXTERNAL:-true}\ndriver: local\n",
		"unknown string":       "external: yes please\ndriver: local\n",
		"false string":         "external: \"false\"\ndriver: local\n",
		"list":                 "external: [true]\ndriver: local\n",
	} {
		t.Run(name, func(t *testing.T) {
			var volume map[string]any
			assert.NilError(t, yaml.Unmarshal([]byte(input), &volume))
			assert.NilError(t, checker(volume, tree.NewPath("volumes.data")))
		})
	}
}

// With interpolation skipped, `external: "true"` is still read as a boolean by
// the decoding of the model: it must keep conflicting with the attributes an
// external resource cannot have.
func TestValidateExternalTrueAsAString(t *testing.T) {
	checker := checks["volumes.*"]
	for _, external := range []string{"true", "TRUE", "yes", "on", "y"} {
		t.Run(external, func(t *testing.T) {
			volume := map[string]any{"external": external, "driver": "local"}
			assert.ErrorContains(t, checker(volume, tree.NewPath("volumes.data")), `conflicting parameters "external" and "driver"`)
		})
	}
}
