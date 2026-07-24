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

package override

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
)

// mergePositional performs an element-by-element (positional) merge for the
// sequences at the given paths instead of the default append. In a compose file
// this is requested with the `!merge` tag on the override sequence; the tests
// below drive the merge directly with explicit paths to illustrate the
// semantics.
func assertPositionalMerge(t *testing.T, right, left, want string, paths ...string) {
	t.Helper()
	pp := make([]tree.Path, len(paths))
	for i, p := range paths {
		pp[i] = tree.NewPath(p)
	}
	got, err := MergeWithPositionalPaths(unmarshal(t, right), unmarshal(t, left), pp)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, want))
}

// A no-op element (`- {}`) keeps the base value at that index; a non-null element
// replaces it. Here only the third argument (the port value) is changed.
func TestMergePositional_TargetsSingleScalar(t *testing.T) {
	base := `
command:
  - server
  - --port
  - "8080"
  - --verbose
`
	override := `
command:
  - {}
  - {}
  - "9090"
`
	want := `
command:
  - server
  - --port
  - "9090"
  - --verbose
`
	assertPositionalMerge(t, base, override, want, "command")
}

// A non-null map element is deep-merged with the base element at the same index,
// so a single field can be changed while the rest of that element is preserved.
func TestMergePositional_DeepMergesMapElement(t *testing.T) {
	base := `
ports:
  - target: 80
    published: "8080"
    protocol: tcp
  - target: 443
    published: "8443"
`
	override := `
ports:
  - published: "9090"
  - {}
`
	want := `
ports:
  - target: 80
    published: "9090"
    protocol: tcp
  - target: 443
    published: "8443"
`
	assertPositionalMerge(t, base, override, want, "ports")
}

// An explicit null element is a no-op too.
func TestMergePositional_NullElementIsNoOp(t *testing.T) {
	base := `
dns:
  - 1.1.1.1
  - 8.8.8.8
`
	override := `
dns:
  - 9.9.9.9
  - ~
`
	want := `
dns:
  - 9.9.9.9
  - 8.8.8.8
`
	assertPositionalMerge(t, base, override, want, "dns")
}

// When the override is longer than the base the sequence is extended; when it is
// shorter the base tail is preserved.
func TestMergePositional_LengthMismatch(t *testing.T) {
	t.Run("override extends base", func(t *testing.T) {
		base := "seq: [a, b]"
		override := "seq: [{}, B, c]"
		want := "seq: [a, B, c]"
		assertPositionalMerge(t, base, override, want, "seq")
	})
	t.Run("base longer than override", func(t *testing.T) {
		base := "seq: [a, b, c]"
		override := "seq: [A]"
		want := "seq: [A, b, c]"
		assertPositionalMerge(t, base, override, want, "seq")
	})
}

// Without a positional path the default append behavior is unchanged — this is
// the contrast that makes `!merge` opt-in and non-breaking.
func TestMergePositional_NotEnabledStillAppends(t *testing.T) {
	base := "seq: [a, b]"
	override := "seq: [c, d]"
	got, err := MergeWithPositionalPaths(unmarshal(t, base), unmarshal(t, override), nil)
	assert.NilError(t, err)
	assert.DeepEqual(t, got, unmarshal(t, "seq: [a, b, c, d]"))
}
