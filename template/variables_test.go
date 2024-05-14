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

package template

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestExtractVariables(t *testing.T) {
	testCases := []struct {
		name     string
		dict     map[string]interface{}
		expected map[string]Variable
	}{
		{
			name:     "empty",
			dict:     map[string]interface{}{},
			expected: map[string]Variable{},
		},
		{
			name: "no-variables",
			dict: map[string]interface{}{
				"foo": "bar",
			},
			expected: map[string]Variable{},
		},
		{
			name: "variable-without-curly-braces",
			dict: map[string]interface{}{
				"foo": "$bar",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar"},
			},
		},
		{
			name: "variable",
			dict: map[string]interface{}{
				"foo": "${bar}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", DefaultValue: ""},
			},
		},
		{
			name: "required-variable",
			dict: map[string]interface{}{
				"foo": "${bar?:foo}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", DefaultValue: "", Required: true},
			},
		},
		{
			name: "required-variable2",
			dict: map[string]interface{}{
				"foo": "${bar?foo}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", DefaultValue: "", Required: true},
			},
		},
		{
			name: "default-variable",
			dict: map[string]interface{}{
				"foo": "${bar:-foo}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", DefaultValue: "foo"},
			},
		},
		{
			name: "default-variable2",
			dict: map[string]interface{}{
				"foo": "${bar-foo}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", DefaultValue: "foo"},
			},
		},
		{
			name: "multiple-values",
			dict: map[string]interface{}{
				"foo": "${bar:-foo}",
				"bar": map[string]interface{}{
					"foo": "${fruit:-banana}",
					"bar": "vegetable",
				},
				"baz": []interface{}{
					"foo",
					"$docker:${project:-cli}",
					"$toto",
				},
			},
			expected: map[string]Variable{
				"bar":     {Name: "bar", DefaultValue: "foo"},
				"fruit":   {Name: "fruit", DefaultValue: "banana"},
				"toto":    {Name: "toto", DefaultValue: ""},
				"docker":  {Name: "docker", DefaultValue: ""},
				"project": {Name: "project", DefaultValue: "cli"},
			},
		},
		{
			name: "presence-value-nonEmpty",
			dict: map[string]interface{}{
				"foo": "${bar:+foo}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", PresenceValue: "foo"},
			},
		},
		{
			name: "presence-value",
			dict: map[string]interface{}{
				"foo": "${bar+foo}",
			},
			expected: map[string]Variable{
				"bar": {Name: "bar", PresenceValue: "foo"},
			},
		},
		{
			name: "concat",
			dict: map[string]interface{}{
				"domainname": "${SUBDOMAIN:-redis}.${ROOTDOMAIN:?}",
			},
			expected: map[string]Variable{
				"ROOTDOMAIN": {Name: "ROOTDOMAIN", Required: true},
				"SUBDOMAIN":  {Name: "SUBDOMAIN", DefaultValue: "redis"},
			},
		},
		{
			name: "nested",
			dict: map[string]interface{}{
				"domainname": "${SUBDOMAIN:-$ROOTDOMAIN}",
			},
			expected: map[string]Variable{
				"ROOTDOMAIN": {Name: "ROOTDOMAIN"},
				"SUBDOMAIN":  {Name: "SUBDOMAIN", DefaultValue: "$ROOTDOMAIN"},
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			actual := ExtractVariables(tc.dict, DefaultPattern)
			assert.Check(t, is.DeepEqual(actual, tc.expected))
		})
	}
}
