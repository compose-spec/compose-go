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
	"sort"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestHostsList(t *testing.T) {
	testCases := []struct {
		doc           string
		input         map[string]any
		expectedError string
		expectedOut   string
	}{
		{
			doc:         "IPv4",
			input:       map[string]any{"myhost": "192.168.0.1"},
			expectedOut: "myhost:192.168.0.1",
		},
		{
			doc:         "Weird but permitted, IPv4 with brackets",
			input:       map[string]any{"myhost": "[192.168.0.1]"},
			expectedOut: "myhost:192.168.0.1",
		},
		{
			doc:         "Host and domain",
			input:       map[string]any{"host.invalid": "10.0.2.1"},
			expectedOut: "host.invalid:10.0.2.1",
		},
		{
			doc:         "IPv6",
			input:       map[string]any{"anipv6host": "2003:ab34:e::1"},
			expectedOut: "anipv6host:2003:ab34:e::1",
		},
		{
			doc:         "IPv6, brackets",
			input:       map[string]any{"anipv6host": "[2003:ab34:e::1]"},
			expectedOut: "anipv6host:2003:ab34:e::1",
		},
		{
			doc:         "IPv6 localhost",
			input:       map[string]any{"ipv6local": "::1"},
			expectedOut: "ipv6local:::1",
		},
		{
			doc:         "IPv6 localhost, brackets",
			input:       map[string]any{"ipv6local": "[::1]"},
			expectedOut: "ipv6local:::1",
		},
		{
			doc:         "host-gateway special case",
			input:       map[string]any{"host.docker.internal": "host-gateway"},
			expectedOut: "host.docker.internal:host-gateway",
		},
		{
			doc: "multiple inputs",
			input: map[string]any{
				"myhost":               "192.168.0.1",
				"anipv6host":           "[2003:ab34:e::1]",
				"host.docker.internal": "host-gateway",
			},
			expectedOut: "anipv6host:2003:ab34:e::1 host.docker.internal:host-gateway myhost:192.168.0.1",
		},
		{
			// This won't work, but address validation is left to the engine.
			doc:         "no ip",
			input:       map[string]any{"myhost": nil},
			expectedOut: "myhost:",
		},
		{
			doc:           "bad host, colon",
			input:         map[string]any{":": "::1"},
			expectedError: "bad host name",
		},
		{
			doc:           "bad host, eq",
			input:         map[string]any{"=": "::1"},
			expectedError: "bad host name",
		},
	}

	inputAsList := func(input map[string]any, sep string) []any {
		result := make([]any, 0, len(input))
		for host, ip := range input {
			if ip == nil {
				result = append(result, host+sep)
			} else {
				result = append(result, host+sep+ip.(string))
			}
		}
		return result
	}

	for _, tc := range testCases {
		// Decode the input map, check the output is as-expected.
		var hlFromMap HostsList
		t.Run(tc.doc+"_map", func(t *testing.T) {
			err := hlFromMap.DecodeMapstructure(tc.input)
			if tc.expectedError == "" {
				assert.NilError(t, err)
				actualOut := hlFromMap.AsList(":")
				sort.Strings(actualOut)
				sortedActualStr := strings.Join(actualOut, " ")
				assert.Check(t, is.Equal(sortedActualStr, tc.expectedOut))

				// The YAML rendering of HostsList should be the same as the AsList() output, but
				// with '=' separators.
				yamlOut, err := hlFromMap.MarshalYAML()
				assert.NilError(t, err)
				expYAMLOut := make([]string, len(actualOut))
				for i, s := range actualOut {
					expYAMLOut[i] = strings.Replace(s, ":", "=", 1)
				}
				assert.DeepEqual(t, yamlOut.([]string), expYAMLOut)

				// The JSON rendering of HostsList should also have '=' separators. Same as the
				// YAML output, but as a JSON list of strings.
				jsonOut, err := hlFromMap.MarshalJSON()
				assert.NilError(t, err)
				expJSONStrings := make([]string, len(expYAMLOut))
				for i, s := range expYAMLOut {
					expJSONStrings[i] = `"` + s + `"`
				}
				expJSONString := "[" + strings.Join(expJSONStrings, ",") + "]"
				assert.Check(t, is.Equal(string(jsonOut), expJSONString))
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})

		// Convert the input into a ':' separated list, check that the result is the same
		// as for the map-input.
		t.Run(tc.doc+"_colon_sep", func(t *testing.T) {
			var hl HostsList
			err := hl.DecodeMapstructure(inputAsList(tc.input, ":"))
			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.DeepEqual(t, hl, hlFromMap)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})

		// Convert the input into a ':' separated list, check that the result is the same
		// as for the map-input.
		t.Run(tc.doc+"_eq_sep", func(t *testing.T) {
			var hl HostsList
			err := hl.DecodeMapstructure(inputAsList(tc.input, "="))
			if tc.expectedError == "" {
				assert.NilError(t, err)
				assert.DeepEqual(t, hl, hlFromMap)
			} else {
				assert.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}
