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

func TestMergePositional(t *testing.T) {
	hook := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	path := tree.Path("services.app.pre_start")

	tests := []struct {
		name     string
		base     []any
		override []any
		expected []any
	}{
		{
			name:     "no-op entries keep the base",
			base:     []any{hook("image", "a"), hook("image", "b")},
			override: []any{hook(), nil},
			expected: []any{hook("image", "a"), hook("image", "b")},
		},
		{
			name:     "entry is merged into the base at the same index",
			base:     []any{hook("image", "a", "command", []any{"x"}), hook("image", "b")},
			override: []any{hook(), hook("image", "b@sha256:1")},
			expected: []any{hook("image", "a", "command", []any{"x"}), hook("image", "b@sha256:1")},
		},
		{
			name:     "command replaces instead of appending",
			base:     []any{hook("command", []any{"migrate", "--up"})},
			override: []any{hook("command", []any{"migrate", "--down"})},
			expected: []any{hook("command", []any{"migrate", "--down"})},
		},
		{
			name:     "environment merges",
			base:     []any{hook("environment", map[string]any{"A": "a"})},
			override: []any{hook("environment", map[string]any{"B": "b"})},
			expected: []any{hook("environment", []any{"A=a", "B=b"})},
		},
		{
			name:     "shorter override keeps the tail of the base",
			base:     []any{hook("image", "a"), hook("image", "b")},
			override: []any{hook("image", "a2")},
			expected: []any{hook("image", "a2"), hook("image", "b")},
		},
		{
			name:     "longer override extends the sequence",
			base:     []any{hook("image", "a")},
			override: []any{nil, hook("image", "c")},
			expected: []any{hook("image", "a"), hook("image", "c")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, err := MergePositional(tt.base, tt.override, path)
			assert.NilError(t, err)
			assert.DeepEqual(t, merged, any(tt.expected))
		})
	}
}

func TestMergePositionalRejectsNonSequence(t *testing.T) {
	_, err := MergePositional([]any{}, map[string]any{}, "services.app.pre_start")
	assert.ErrorContains(t, err, "override is not a sequence")
	_, err = MergePositional("x", []any{}, "services.app.pre_start")
	assert.ErrorContains(t, err, "base is not a sequence")
}
