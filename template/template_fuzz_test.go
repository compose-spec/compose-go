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
	"reflect"
	"strings"
	"testing"
)

// templateSeeds are the interpolation forms of the Compose specification
// (https://github.com/compose-spec/compose-spec/blob/main/12-interpolation.md):
// plain and braced variables, default, alternative and required values (with
// and without the `:` variant), the `$$` escape, nested expansions and
// malformed templates.
var templateSeeds = []string{
	"",
	"plain text",
	"$FOO",
	"${FOO}",
	"$$FOO",
	"$${FOO}",
	"$$$FOO",
	"${FOO:-default}",
	"${FOO-default}",
	"${EMPTY:-default}",
	"${EMPTY-default}",
	"${FOO:+alt}",
	"${FOO+alt}",
	"${EMPTY:+alt}",
	"${UNSET:?missing}",
	"${UNSET?missing}",
	"${EMPTY:?missing}",
	"${UNSET:-${FOO:-nested}}",
	"${UNSET:-${UNSET:-${UNSET:-deep}}}",
	"${UNSET:?${FOO}}",
	"${FOO:-a}-${FOO:+b}-$FOO",
	"${FOO:-{}}",
	"${FOO:-}}",
	"${",
	"${}",
	"${FOO",
	"${FOO:-${BAR}",
	"${1FOO}",
	"$",
	"$ ",
	"${FOO:-$$}",
	"${FOO:-$}",
	"é${FOO}é",
}

// fuzzMapping is a fixed environment: one regular, one empty variable and
// everything else unset.
func fuzzMapping(name string) (string, bool) {
	switch name {
	case "FOO":
		return "foo", true
	case "BAR":
		return "bar", true
	case "EMPTY":
		return "", true
	}
	return "", false
}

// FuzzSubstitute locks the safety of interpolation
// (https://github.com/compose-spec/compose-spec/blob/main/12-interpolation.md):
// whatever the template, Substitute must return a result or an error, never
// panic, and must be deterministic. A template with no `$` has nothing to
// interpolate and comes back untouched. Escaping every `$` as `$$` turns any
// text into a template that interpolates to itself.
func FuzzSubstitute(f *testing.F) {
	for _, s := range templateSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tmpl string) {
		got, err := SubstituteWithOptions(tmpl, fuzzMapping, WithoutLogging)
		again, err2 := SubstituteWithOptions(tmpl, fuzzMapping, WithoutLogging)
		if got != again || (err == nil) != (err2 == nil) {
			t.Fatalf("Substitute(%q) is not deterministic: (%q, %v) then (%q, %v)", tmpl, got, err, again, err2)
		}

		if !strings.Contains(tmpl, "$") {
			if err != nil || got != tmpl {
				t.Fatalf("Substitute(%q) = (%q, %v), want the template untouched", tmpl, got, err)
			}
		}

		escaped := strings.ReplaceAll(tmpl, "$", "$$")
		got, err = SubstituteWithOptions(escaped, fuzzMapping, WithoutLogging)
		if err != nil || got != tmpl {
			t.Fatalf("Substitute(%q) = (%q, %v), want %q back", escaped, got, err, tmpl)
		}
	})
}

// FuzzExtractVariables locks that listing the variables of a document, whatever
// the (possibly malformed) templates it holds, never panics and stays
// deterministic.
func FuzzExtractVariables(f *testing.F) {
	for _, s := range templateSeeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, tmpl string) {
		doc := map[string]interface{}{
			"scalar": tmpl,
			"list":   []interface{}{tmpl, 1, nil},
			"map":    map[string]interface{}{"k": tmpl},
		}
		first := ExtractVariables(doc, nil)
		second := ExtractVariables(doc, nil)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("ExtractVariables(%q) is not deterministic: %v then %v", tmpl, first, second)
		}
	})
}
