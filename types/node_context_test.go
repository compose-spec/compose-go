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

package types

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestOrigin_String(t *testing.T) {
	assert.Equal(t, Origin{}.String(), "")
	assert.Equal(t, Origin{Source: "compose.yaml"}.String(), "compose.yaml")
	assert.Equal(t, Origin{Source: "compose.yaml", Line: 12}.String(), "compose.yaml:12")
	assert.Equal(t, Origin{Source: "compose.yaml", Line: 12, Column: 5}.String(), "compose.yaml:12:5")
}

func TestNodeContext_OriginAt(t *testing.T) {
	ctx := &NodeContext{Source: "compose.yaml"}
	node := &yaml.Node{Line: 7, Column: 3}
	o := ctx.OriginAt(node)
	assert.Equal(t, o.Source, "compose.yaml")
	assert.Equal(t, o.Line, 7)
	assert.Equal(t, o.Column, 3)
	assert.Equal(t, o.String(), "compose.yaml:7:3")
}

func TestNodeContext_OriginAt_NilContext(t *testing.T) {
	var ctx *NodeContext
	node := &yaml.Node{Line: 4, Column: 1}
	o := ctx.OriginAt(node)
	assert.Equal(t, o.Source, "")
	assert.Equal(t, o.Line, 4)
}

func TestNodeContext_OriginAt_NilNode(t *testing.T) {
	ctx := &NodeContext{Source: "compose.yaml"}
	o := ctx.OriginAt(nil)
	assert.Equal(t, o.Source, "compose.yaml")
	assert.Equal(t, o.Line, 0)
	assert.Equal(t, o.String(), "compose.yaml")
}
