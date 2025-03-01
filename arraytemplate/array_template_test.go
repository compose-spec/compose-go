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

package arraytemplate

import (
	"fmt"
	"gotest.tools/v3/assert"
	"testing"
)

func buildMapping(mapping map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := mapping[name]
		return v, ok
	}
}

func TestIndexedArraysSubstitutions(t *testing.T) {
	testcases := []struct {
		template      string
		mapping       map[string]string
		expectedArray []string
	}{
		{
			template: "$INDEXED_ARRAY[*]",
			mapping: map[string]string{
				"INDEXED_ARRAY[0]": "zero",
				"INDEXED_ARRAY[1]": "one",
				"INDEXED_ARRAY[2]": "two",
			},
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			template: "${BRACED_INDEXED_ARRAY[*]}",
			mapping: map[string]string{
				"BRACED_INDEXED_ARRAY[0]": "zero",
				"BRACED_INDEXED_ARRAY[1]": "one",
				"BRACED_INDEXED_ARRAY[2]": "two",
			},
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			template: "$EMPTY_ARRAY[*]",
			mapping: map[string]string{
				"NON_RELEVANT": "WHATEVER",
			},
			expectedArray: []string{},
		},
		{
			template: "$MISINDEXED_ARRAY[*]",
			mapping: map[string]string{
				"MISINDEXED_ARRAY[1]": "zero",
				"MISINDEXED_ARRAY[2]": "one",
				"MISINDEXED_ARRAY[3]": "two",
			},
			expectedArray: []string{},
		},
		{
			template: "$INTERRUPTED_ARRAY[*]",
			mapping: map[string]string{
				"INTERRUPTED_ARRAY[0]": "zero",
				"INTERRUPTED_ARRAY[1":  "one",
				"INTERRUPTED_ARRAY[2]": "two",
			},
			expectedArray: []string{"zero"},
		},
		{
			template: "$INVALID_KEY[*]",
			mapping: map[string]string{
				"INDEXED_ARRAY[0]": "zero",
				"INDEXED_ARRAY[1]": "one",
				"INDEXED_ARRAY[2]": "two",
			},
			expectedArray: []string{},
		},
		{
			template: "$ASTERIX_ARRAY[*]",
			mapping: map[string]string{
				"ASTERIX_ARRAY[*]": "asterix",
			},
			expectedArray: []string{},
		},
	}

	for _, testcase := range testcases {
		mapping := buildMapping(testcase.mapping)
		result, err := Substitute(testcase.template, mapping)
		assert.NilError(t, err)
		assert.DeepEqual(t, testcase.expectedArray, result)
	}
}

func TestInlinedArraysSubstitutions(t *testing.T) {
	testcases := []struct {
		rawValue      string
		expectedArray []string
	}{
		{
			rawValue:      "(zero one two)",
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			rawValue:      "()",
			expectedArray: []string{},
		},
		{
			rawValue:      "( )",
			expectedArray: []string{},
		},
		{
			rawValue:      "(\"zero\" \"one\" \"two\")",
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			rawValue:      "('zero' 'one' 'two')",
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			rawValue:      "(\"zero 0\" \"one 1\" \"two 2\")",
			expectedArray: []string{"zero 0", "one 1", "two 2"},
		},
		{
			rawValue:      "(zero\\ 0 one\\ 1 two\\ 2)",
			expectedArray: []string{"zero 0", "one 1", "two 2"},
		},
		{
			rawValue:      "(zero\\ 0 \"one 1\" two\\ 2)",
			expectedArray: []string{"zero 0", "one 1", "two 2"},
		},
		{
			rawValue:      "(  zero   one   two  )",
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			rawValue:      "( '\"' )",
			expectedArray: []string{"\""},
		},
		{
			rawValue:      "( \"'\" )",
			expectedArray: []string{"'"},
		},
		{
			rawValue:      "( \\' )",
			expectedArray: []string{"'"},
		},
		{
			rawValue:      "( \\\" )",
			expectedArray: []string{"\""},
		},
		{
			rawValue:      "( \"\\\"\" )",
			expectedArray: []string{"\""},
		},
		{
			rawValue:      "( '\\'' )",
			expectedArray: []string{"'"},
		},
		{
			rawValue:      "(\\\\)",
			expectedArray: []string{"\\"},
		},
	}

	for _, testcase := range testcases {
		mapping := buildMapping(map[string]string{"arr": testcase.rawValue})
		result, err := Substitute("$arr[*]", mapping)
		assert.NilError(t, err)
		assert.DeepEqual(t, testcase.expectedArray, result)
	}
}

func TestInlinedPriority(t *testing.T) {
	testcases := []struct {
		template      string
		mapping       map[string]string
		expectedArray []string
	}{
		{
			template: "$MIXED_ARRAY[*]",
			mapping: map[string]string{
				"MIXED_ARRAY":    "(zero one two)",
				"MIXED_ARRAY[0]": "0",
				"MIXED_ARRAY[1]": "1",
				"MIXED_ARRAY[2]": "2",
			},
			expectedArray: []string{"zero", "one", "two"},
		},
		{
			template: "$MIXED_ARRAY[*]",
			mapping: map[string]string{
				"MIXED_ARRAY":    "(zero one two)",
				"MIXED_ARRAY[3]": "3",
			},
			expectedArray: []string{"zero", "one", "two"},
		},
	}

	for _, testcase := range testcases {
		mapping := buildMapping(testcase.mapping)
		result, err := Substitute(testcase.template, mapping)
		assert.NilError(t, err)
		assert.DeepEqual(t, testcase.expectedArray, result)
	}
}

func TestBadInlinedDeclarations(t *testing.T) {
	expectedErrMsg := func(value string, cause string) string {
		return fmt.Sprintf(
			"could not substitute array template \"$arr[*]\":\ninvalid array definition: \"%s\" - %s",
			value,
			cause,
		)
	}
	testcases := []struct {
		value string
		cause string
	}{
		{value: "(", cause: "should be enclosed in parenthesis"},
		{value: ")", cause: "should be enclosed in parenthesis"},
		{value: "abc", cause: "should be enclosed in parenthesis"},
		{value: "(zero", cause: "should be enclosed in parenthesis"},
		{value: "zero)", cause: "should be enclosed in parenthesis"},
		{value: "(zero one", cause: "should be enclosed in parenthesis"},
		{value: "one zero)", cause: "should be enclosed in parenthesis"},
		{value: "(zero one ", cause: "should be enclosed in parenthesis"},
		{value: " one zero)", cause: "should be enclosed in parenthesis"},
		{value: "(\")", cause: "quote not closed"},
		{value: "(')", cause: "quote not closed"},
		{value: "(zero one))", cause: "unescaped character (\")\")"},
		{value: "((zero one)", cause: "unescaped character (\"(\")"},
		{value: "(\"one)", cause: "quote not closed"},
		{value: "(one\")", cause: "quote not closed"},
		{value: "('one)", cause: "quote not closed"},
		{value: "(one')", cause: "quote not closed"},
		{value: "(\\)", cause: "nothing left to escape"},
	}

	for _, testcase := range testcases {
		mapping := buildMapping(map[string]string{"arr": testcase.value})
		result, err := Substitute("$arr[*]", mapping)
		assert.DeepEqual(t, result, []string(nil))
		assert.Error(t, err, expectedErrMsg(testcase.value, testcase.cause))
	}
}

func TestBadTemplates(t *testing.T) {
	testcases := []string{
		"NO_DOLLAR_SIGN[*]",
		"$NO_CLOSING_BRACKET[*",
		"$NO_OPENING_BRACKET*]",
		"$NO_OPENING_BRACE[*]}",
		"${NO_CLOSING_BRACE[*]",
		"${INDEX_OUTSIZE_BRACES}[*]",
	}

	for _, testcase := range testcases {
		neverMapping := func(name string) (string, bool) { return "", false }
		result, err := Substitute(testcase, neverMapping)
		assert.DeepEqual(t, result, []string(nil))
		assert.Error(t, err, fmt.Sprintf("not a valid array template: \"%s\"", testcase))
	}
}
