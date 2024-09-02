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

func TestHostsListEqual(t *testing.T) {
	testHostsList(t, "=")
}

func TestHostsListComa(t *testing.T) {
	testHostsList(t, ":")
}

func testHostsList(t *testing.T, sep string) {
	testCases := []struct {
		doc           string
		input         []string
		expectedError string
		expectedOut   string
	}{
		{
			doc:         "IPv4",
			input:       []string{"myhost" + sep + "192.168.0.1"},
			expectedOut: "myhost:192.168.0.1",
		},
		{
			doc:         "Weird but permitted, IPv4 with brackets",
			input:       []string{"myhost" + sep + "[192.168.0.1]"},
			expectedOut: "myhost:192.168.0.1",
		},
		{
			doc:         "Host and domain",
			input:       []string{"host.invalid" + sep + "10.0.2.1"},
			expectedOut: "host.invalid:10.0.2.1",
		},
		{
			doc:         "IPv6",
			input:       []string{"anipv6host" + sep + "2003:ab34:e::1"},
			expectedOut: "anipv6host:2003:ab34:e::1",
		},
		{
			doc:         "IPv6, brackets",
			input:       []string{"anipv6host" + sep + "[2003:ab34:e::1]"},
			expectedOut: "anipv6host:2003:ab34:e::1",
		},
		{
			doc:         "IPv6 localhost",
			input:       []string{"ipv6local" + sep + "::1"},
			expectedOut: "ipv6local:::1",
		},
		{
			doc:         "IPv6 localhost, brackets",
			input:       []string{"ipv6local" + sep + "[::1]"},
			expectedOut: "ipv6local:::1",
		},
		{
			doc:         "host-gateway special case",
			input:       []string{"host.docker.internal" + sep + "host-gateway"},
			expectedOut: "host.docker.internal:host-gateway",
		},
		{
			doc: "multiple inputs",
			input: []string{
				"myhost" + sep + "192.168.0.1",
				"anipv6host" + sep + "[2003:ab34:e::1]",
				"host.docker.internal" + sep + "host-gateway",
			},
			expectedOut: "anipv6host:2003:ab34:e::1 host.docker.internal:host-gateway myhost:192.168.0.1",
		},
		{
			doc:           "bad host, colon",
			input:         []string{"::::1"},
			expectedError: "bad host name",
		},
		{
			doc:           "bad host, eq",
			input:         []string{"=::1"},
			expectedError: "bad host name",
		},
		{
			doc: "both ipv4 and ipv6",
			input: []string{
				"foo:127.0.0.2",
				"foo:ff02::1",
			},
			expectedOut: "foo:127.0.0.2 foo:ff02::1",
		},
		{
			doc: "list of values",
			input: []string{
				"foo=127.0.0.2,127.0.0.3",
			},
			expectedOut: "foo:127.0.0.2 foo:127.0.0.3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.doc, func(t *testing.T) {
			hostlist, err := NewHostsList(tc.input)
			if tc.expectedError == "" {
				assert.NilError(t, err)
				actualOut := hostlist.AsList(":")
				sort.Strings(actualOut)
				sortedActualStr := strings.Join(actualOut, " ")
				assert.Check(t, is.Equal(sortedActualStr, tc.expectedOut))

				// The YAML rendering of HostsList should be the same as the AsList() output, but
				// with '=' separators.
				yamlOut, err := hostlist.MarshalYAML()
				assert.NilError(t, err)
				expYAMLOut := make([]string, len(actualOut))
				for i, s := range actualOut {
					expYAMLOut[i] = strings.Replace(s, ":", "=", 1)
				}
				assert.DeepEqual(t, yamlOut.([]string), expYAMLOut)

				// The JSON rendering of HostsList should also have '=' separators. Same as the
				// YAML output, but as a JSON list of strings.
				jsonOut, err := hostlist.MarshalJSON()
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
	}
}
