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

package loader

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestOmitEmptyPreservesEmptySlice(t *testing.T) {
	input := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"build": map[string]any{
					"cache_to": []any{},
				},
			},
		},
	}
	got := OmitEmpty(input)
	cacheTo := got["services"].(map[string]any)["foo"].(map[string]any)["build"].(map[string]any)["cache_to"]
	slice, ok := cacheTo.([]any)
	assert.Assert(t, ok, "cache_to should remain a []any, got %T", cacheTo)
	assert.Assert(t, slice != nil, "cache_to should remain a non-nil empty slice")
	assert.Equal(t, len(slice), 0)
}
