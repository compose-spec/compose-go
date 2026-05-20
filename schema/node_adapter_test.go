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

package schema

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func parseDoc(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &n))
	return &n
}

func TestValidateNode_AcceptsValidCompose(t *testing.T) {
	doc := parseDoc(t, `
services:
  app:
    image: alpine
`)
	assert.NilError(t, ValidateNode(doc))
}

func TestValidateNode_RejectsInvalidShape(t *testing.T) {
	doc := parseDoc(t, `
services: not-a-mapping
`)
	err := ValidateNode(doc)
	assert.Assert(t, err != nil, "expected validation error")
}

func TestNodeToInterface_ScalarTypes(t *testing.T) {
	doc := parseDoc(t, `
str: hello
int: 42
float: 3.14
bool: true
null_val: null
seq:
  - a
  - 1
mapping:
  inner: x
`)
	v, err := nodeToInterface(doc)
	assert.NilError(t, err)
	m := v.(map[string]any)
	assert.Equal(t, m["str"], "hello")
	assert.Equal(t, m["int"], int(42))
	assert.Equal(t, m["float"], 3.14)
	assert.Equal(t, m["bool"], true)
	assert.Equal(t, m["null_val"], nil)
	seq := m["seq"].([]any)
	assert.Equal(t, seq[0], "a")
	assert.Equal(t, seq[1], int(1))
	inner := m["mapping"].(map[string]any)
	assert.Equal(t, inner["inner"], "x")
}

func TestValidateNode_RejectsNonMappingTopLevel(t *testing.T) {
	doc := parseDoc(t, `
- a
- b
`)
	err := ValidateNode(doc)
	assert.ErrorContains(t, err, "must be a mapping")
}
