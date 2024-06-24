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

package tree

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPathMatches(t *testing.T) {
	var testcases = []struct {
		doc      string
		path     Path
		pattern  Path
		expected bool
	}{
		{
			doc:     "pattern too short",
			path:    NewPath("one", "two", "three"),
			pattern: NewPath("one", "two"),
		},
		{
			doc:     "pattern too long",
			path:    NewPath("one", "two"),
			pattern: NewPath("one", "two", "three"),
		},
		{
			doc:     "pattern mismatch",
			path:    NewPath("one", "three", "two"),
			pattern: NewPath("one", "two", "three"),
		},
		{
			doc:     "pattern mismatch with match-all part",
			path:    NewPath("one", "three", "two"),
			pattern: NewPath(PathMatchAll, "two", "three"),
		},
		{
			doc:      "pattern match with match-all part",
			path:     NewPath("one", "two", "three"),
			pattern:  NewPath("one", "*", "three"),
			expected: true,
		},
		{
			doc:      "pattern match",
			path:     NewPath("one", "two", "three"),
			pattern:  NewPath("one", "two", "three"),
			expected: true,
		},
		{
			doc:      "any at the beginning",
			path:     NewPath("one", "two", "three"),
			pattern:  NewPath("**", "three"),
			expected: true,
		},
		{
			doc:      "any at the beginning followed by a wrong part",
			path:     NewPath("one", "two", "three"),
			pattern:  NewPath("**", "four"),
			expected: false,
		},
		{
			doc:      "any in the middle",
			path:     NewPath("one", "two", "three", "four"),
			pattern:  NewPath("one", "**", "four"),
			expected: true,
		},
		{
			doc:      "any in the middle followed by a wrong part",
			path:     NewPath("one", "two", "three", "four"),
			pattern:  NewPath("one", "**", "five"),
			expected: false,
		},
		{
			doc:      "any at the end",
			path:     NewPath("one", "two", "three", "four", "five", "six"),
			pattern:  NewPath("one", "two", "three", "**"),
			expected: true,
		},
	}
	for _, testcase := range testcases {
		assert.Assert(t, is.Equal(testcase.expected, testcase.path.Matches(testcase.pattern)), testcase.doc)
	}
}

func TestPathMatchesAny(t *testing.T) {
	var testcases = []struct {
		doc      string
		path     Path
		patterns []Path
		expected bool
	}{
		{
			doc:      "empty patterns slice should return true",
			path:     NewPath("one", "two", "three"),
			patterns: nil,
			expected: true,
		},
		{
			doc:      "match with one pattern containing *",
			path:     NewPath("one", "two", "three"),
			patterns: []Path{NewPath("one", "*", "three")},
			expected: true,
		},
		{
			doc:      "match with multiple patterns including",
			path:     NewPath("one", "two", "three"),
			patterns: []Path{NewPath("one", "*", "three"), NewPath("one", "two", "four"), NewPath("**", "two", "four")},
			expected: true,
		},
		{
			doc:      "match with ** at the beginning",
			path:     NewPath("one", "two", "three"),
			patterns: []Path{NewPath("**", "three")},
			expected: true,
		},
		{
			doc:      "match with ** in the middle",
			path:     NewPath("one", "two", "three", "four"),
			patterns: []Path{NewPath("one", "**", "four")},
			expected: true,
		},
		{
			doc:      "match with ** at the end",
			path:     NewPath("one", "two", "three", "four", "five", "six"),
			patterns: []Path{NewPath("one", "two", "three", "**")},
			expected: true,
		},
		{
			doc:  "no match with any pattern",
			path: NewPath("one", "two", "three"),
			patterns: []Path{
				NewPath("one", "four", "three"),
				NewPath("not one", "**", "three"),
				NewPath("not one", "**", "three"),
			},
			expected: false,
		},
		{
			doc:      "empty path should not match any non-empty pattern",
			path:     NewPath(""),
			patterns: []Path{NewPath("one")},
			expected: false,
		},
		{
			doc:      "empty path should match empty pattern",
			path:     NewPath(""),
			patterns: []Path{NewPath("")},
			expected: true,
		},
		{
			doc:      "empty pattern should match empty path",
			path:     NewPath(""),
			patterns: []Path{NewPath("")},
			expected: true,
		},
	}

	for _, tc := range testcases {
		assert.Equal(t, tc.expected, tc.path.MatchesAny(tc.patterns), tc.doc)
	}
}
