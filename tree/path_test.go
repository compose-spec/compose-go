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
	testcases := []struct {
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
	}
	for _, testcase := range testcases {
		t.Run(testcase.doc, func(t *testing.T) {
			assert.Check(t, is.Equal(testcase.expected, testcase.path.Matches(testcase.pattern)))
		})
	}
}

// matchesBySplit is the reference definition of Path.Matches: both paths are
// split into segments, and must have the same depth with every pattern segment
// either PathMatchAll or equal to the path one.
func matchesBySplit(p, pattern Path) bool {
	patternParts := pattern.Parts()
	parts := p.Parts()
	if len(patternParts) != len(parts) {
		return false
	}
	for index, part := range parts {
		switch patternParts[index] {
		case PathMatchAll, part:
			continue
		default:
			return false
		}
	}
	return true
}

var pathMatchesSeeds = []struct{ path, pattern Path }{
	{"", ""},
	{"", "*"},
	{"a", ""},
	{"", "a"},
	{"a", "a"},
	{"a", "*"},
	{"a", "b"},
	{"a.", "a."},
	{"a.", "a"},
	{"a", "a."},
	{"a.", "*."},
	{".a", "*.a"},
	{"a..b", "a.*.b"},
	{"a..b", "a.b"},
	{"a.b", "a..b"},
	{"services.web.ports.[]", "services.*.ports.[]"},
	{"services.web.ports.[]", "services.*.ports.*"},
	{"services.web.ports.[]", "services.*.ports"},
	{"services.web", "services.*.ports.[]"},
	{"services.web.environment.DEBUG", "services.*.environment.*"},
	{"services.web.environment.DEBUG", "*"},
	{"x.*", "x.y"},
	{"x.*", "x.*"},
	{NewPath("services", "web").Next("a.b"), "services.web.*"},
	{NewPath("services", "web").Next("a.b"), "services.web.a.b"},
}

// Path.Matches must answer exactly what the definition by segment-wise
// comparison of the split paths answers: the loader relies on it to pick the
// transformation, default, check or merge rule of every node of a document.
func TestPathMatchesEquivalence(t *testing.T) {
	for _, seed := range pathMatchesSeeds {
		t.Run(string(seed.path)+" ~ "+string(seed.pattern), func(t *testing.T) {
			assert.Check(t, is.Equal(matchesBySplit(seed.path, seed.pattern), seed.path.Matches(seed.pattern)))
		})
	}
}

// Path.Matches runs for every node against every pattern of a table: it must
// not allocate, or the garbage it produces dominates the cost of loading a
// large Compose file.
func TestPathMatchesDoesNotAllocate(t *testing.T) {
	path := NewPath("services", "web", "deploy", "resources", "limits", "memory")
	patterns := []Path{
		"services.*.deploy.resources.limits.memory", // match
		"services.*.deploy.resources.limits.cpus",   // same depth, last segment differs
		"services.*.ports.[]",                       // shorter
		"services.*.deploy.resources.limits.memory.x",
	}
	allocs := testing.AllocsPerRun(100, func() {
		for _, pattern := range patterns {
			path.Matches(pattern)
		}
	})
	assert.Check(t, is.Equal(allocs, 0.0))
}

// Path.Matches is the segment-wise comparison of the split paths, for any
// input: the allocation-free walk must never diverge from the definition.
func FuzzPathMatches(f *testing.F) {
	for _, seed := range pathMatchesSeeds {
		f.Add(string(seed.path), string(seed.pattern))
	}
	f.Fuzz(func(t *testing.T, path, pattern string) {
		if got, want := Path(path).Matches(Path(pattern)), matchesBySplit(Path(path), Path(pattern)); got != want {
			t.Errorf("Path(%q).Matches(%q) = %v, want %v", path, pattern, got, want)
		}
	})
}

func BenchmarkPathMatches(b *testing.B) {
	path := NewPath("services", "web", "deploy", "resources", "limits", "memory")
	patterns := map[string]Path{
		"match":          "services.*.deploy.resources.limits.memory",
		"differs-early":  "networks.*.driver_opts.*.x.y",
		"differs-late":   "services.*.deploy.resources.limits.cpus",
		"different-size": "services.*.ports.[]",
	}
	for name, pattern := range patterns {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				path.Matches(pattern)
			}
		})
	}
}
