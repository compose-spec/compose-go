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

package utils

import (
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSet_Has(t *testing.T) {
	x := NewSet[string]("value")
	assert.Assert(t, x.Has("value"))
	assert.Assert(t, !x.Has("VALUE"))
}

func TestSet_Diff(t *testing.T) {
	a := NewSet[int](1, 2)
	b := NewSet[int](2, 3)

	assert.Check(t, is.DeepEqual(a.Diff(b), NewSet[int](1)))
	assert.Check(t, is.DeepEqual(b.Diff(a), NewSet[int](3)))
}

func TestSet_Union(t *testing.T) {
	a := NewSet[int](1, 2)
	b := NewSet[int](2, 3)

	expected := NewSet[int](1, 2, 3)
	assert.Check(t, is.DeepEqual(a.Union(b), expected))
	assert.Check(t, is.DeepEqual(b.Union(a), expected))
}
