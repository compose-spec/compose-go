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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParsePortConfig(t *testing.T) {
	testCases := []struct {
		value         string
		expectedError string
		expected      []ServicePortConfig
	}{
		{
			value: "80",
			expected: []ServicePortConfig{
				{
					Protocol: "tcp",
					Target:   80,
					Mode:     "ingress",
				},
			},
		},
		{
			value: "80:8080",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: "80",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-90:8080",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: "80-90",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "8080:80/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    80,
					Published: "8080",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80:8080/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: "80",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-81:8080-8081/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: "80",
					Mode:      "ingress",
				},
				{
					Protocol:  "tcp",
					Target:    8081,
					Published: "81",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-82:8080-8082/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: "80",
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8081,
					Published: "81",
					Mode:      "ingress",
				},
				{
					Protocol:  "udp",
					Target:    8082,
					Published: "82",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-82:8080/udp",
			expected: []ServicePortConfig{
				{
					Protocol:  "udp",
					Target:    8080,
					Published: "80-82",
					Mode:      "ingress",
				},
			},
		},
		{
			value: "80-80:8080/tcp",
			expected: []ServicePortConfig{
				{
					Protocol:  "tcp",
					Target:    8080,
					Published: "80",
					Mode:      "ingress",
				},
			},
		},
		{
			value:         "9999999",
			expectedError: "Invalid containerPort: 9999999",
		},
		{
			value:         "80/xyz",
			expectedError: "Invalid proto: xyz",
		},
		{
			value:         "tcp",
			expectedError: "Invalid containerPort: tcp",
		},
		{
			value:         "udp",
			expectedError: "Invalid containerPort: udp",
		},
		{
			value:         "",
			expectedError: "No port specified: <empty>",
		},
		{
			value: "1.1.1.1:80:80",
			expected: []ServicePortConfig{
				{
					HostIP:    "1.1.1.1",
					Protocol:  "tcp",
					Target:    80,
					Published: "80",
					Mode:      "ingress",
				},
			},
		},
	}
	for _, tc := range testCases {
		ports, err := ParsePortConfig(tc.value)
		if tc.expectedError != "" {
			assert.Error(t, err, tc.expectedError)
			continue
		}
		assert.NilError(t, err)
		assert.Check(t, is.Len(ports, len(tc.expected)))
		for _, expectedPortConfig := range tc.expected {
			assertContains(t, ports, expectedPortConfig)
		}
	}
}

func assertContains(t *testing.T, portConfigs []ServicePortConfig, expected ServicePortConfig) {
	var contains = false
	for _, portConfig := range portConfigs {
		if is.DeepEqual(portConfig, expected)().Success() {
			contains = true
			break
		}
	}
	if !contains {
		t.Errorf("expected %v to contain %v, did not", portConfigs, expected)
	}
}

type foo struct {
	Bar string
}

func TestExtension(t *testing.T) {
	x := Extensions{
		"foo": map[string]interface{}{
			"bar": "zot",
		},
	}
	var foo foo
	ok, err := x.Get("foo", &foo)
	assert.NilError(t, err)
	assert.Check(t, ok == true)
	assert.Check(t, foo.Bar == "zot")

	ok, err = x.Get("qiz", &foo)
	assert.NilError(t, err)
	assert.Check(t, ok == false)
}

func TestNewMapping(t *testing.T) {
	m := NewMapping([]string{
		"FOO=BAR",
		"ZOT=",
		"QIX",
	})
	mw := NewMappingWithEquals([]string{
		"FOO=BAR",
		"ZOT=",
		"QIX",
	})
	assert.Check(t, m["FOO"] == "BAR")
	assert.Check(t, m["ZOT"] == "")
	assert.Check(t, m["QIX"] == "")
	assert.Check(t, *mw["FOO"] == "BAR")
	assert.Check(t, *mw["ZOT"] == "")
	assert.Check(t, mw["QIX"] == nil)
}

func TestNetworksByPriority(t *testing.T) {
	s := ServiceConfig{
		Networks: map[string]*ServiceNetworkConfig{
			"foo": nil,
			"bar": {
				Priority: 10,
			},
			"zot": {
				Priority: 100,
			},
			"qix": {
				Priority: 1000,
			},
		},
	}
	assert.DeepEqual(t, s.NetworksByPriority(), []string{"qix", "zot", "bar", "foo"})
}

func TestNetworksByPriorityWithEqualPriorities(t *testing.T) {
	s := ServiceConfig{
		Networks: map[string]*ServiceNetworkConfig{
			"foo": nil,
			"bar": nil,
			"zot": nil,
			"qix": nil,
		},
	}
	assert.DeepEqual(t, s.NetworksByPriority(), []string{"bar", "foo", "qix", "zot"})
}

func TestMarshalServiceEntrypoint(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name         string
		entrypoint   ShellCommand
		expectedYAML string
		expectedJSON string
	}{
		{
			name:         "nil",
			entrypoint:   nil,
			expectedYAML: `{}`,
			expectedJSON: `{"command":null,"entrypoint":null}`,
		},
		{
			name:         "empty",
			entrypoint:   make([]string, 0),
			expectedYAML: `entrypoint: []`,
			expectedJSON: `{"command":null,"entrypoint":[]}`,
		},
		{
			name:         "value",
			entrypoint:   ShellCommand{"ls", "/"},
			expectedYAML: "entrypoint:\n    - ls\n    - /",
			expectedJSON: `{"command":null,"entrypoint":["ls","/"]}`,
		},
	}

	assertEqual := func(t testing.TB, actualBytes []byte, expected string) {
		t.Helper()
		actual := strings.TrimSpace(string(actualBytes))
		expected = strings.TrimSpace(expected)
		assert.Equal(t, actual, expected)
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := ServiceConfig{Entrypoint: tc.entrypoint}
			actualYAML, err := yaml.Marshal(s)
			assert.NilError(t, err, "YAML marshal failed")
			assertEqual(t, actualYAML, tc.expectedYAML)

			actualJSON, err := json.Marshal(s)
			assert.NilError(t, err, "JSON marshal failed")
			assertEqual(t, actualJSON, tc.expectedJSON)
		})
	}

}

func TestMarshalBuild_DockerfileInline(t *testing.T) {
	b := BuildConfig{
		DockerfileInline: "FROM alpine\n\n# echo the env\nRUN env\n\nENTRYPOINT /bin/echo\n",
	}
	out, err := yaml.Marshal(b)
	assert.NilError(t, err)

	const expected = `
dockerfile_inline: |
    FROM alpine

    # echo the env
    RUN env

    ENTRYPOINT /bin/echo
`
	assert.Check(t, equalTrimSpace(out, expected))

	// round-trip
	var b2 BuildConfig
	assert.NilError(t, yaml.Unmarshal(out, &b2))
	assert.Check(t, equalTrimSpace(b.DockerfileInline, b2.DockerfileInline))
}

func equalTrimSpace(x interface{}, y interface{}) is.Comparison {
	trim := func(v interface{}) interface{} {
		switch vv := v.(type) {
		case string:
			return strings.TrimSpace(vv)
		case []byte:
			return string(bytes.TrimSpace(vv))
		}
		panic(fmt.Errorf("invalid type %T (value: %+v)", v, v))
	}
	return is.DeepEqual(trim(x), trim(y))
}

func TestMappingValues(t *testing.T) {
	values := []string{"BAR=QIX", "FOO=BAR", "QIX=ZOT"}
	mapping := NewMapping(values)
	assert.DeepEqual(t, mapping, Mapping{
		"FOO": "BAR",
		"BAR": "QIX",
		"QIX": "ZOT",
	})
	assert.DeepEqual(t, mapping.Values(), values)
}
