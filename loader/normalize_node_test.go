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

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/types"
)

func parseNormalizeNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func decodeNormalize(t *testing.T, n *yaml.Node) map[string]any {
	t.Helper()
	var m map[string]any
	assert.NilError(t, n.Decode(&m))
	return m
}

func TestNormalizeNode_InjectsDefaultNetwork(t *testing.T) {
	root := parseNormalizeNode(t, `
name: app
services:
  web:
    image: nginx
`)
	out, err := NormalizeNode(root, types.Mapping{})
	assert.NilError(t, err)
	m := decodeNormalize(t, out)
	nets, ok := m["networks"].(map[string]any)
	assert.Assert(t, ok, "default network injected: %v", m["networks"])
	_, hasDefault := nets["default"]
	assert.Assert(t, hasDefault, "networks.default should be created")
}

func TestNormalizeNode_BuildContextDefaultsToDot(t *testing.T) {
	root := parseNormalizeNode(t, `
name: app
services:
  web:
    build:
      dockerfile: Dockerfile
`)
	out, err := NormalizeNode(root, types.Mapping{})
	assert.NilError(t, err)
	m := decodeNormalize(t, out)
	build := m["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], ".")
}

func TestNormalizeNode_NilSafe(t *testing.T) {
	out, err := NormalizeNode(nil, types.Mapping{})
	assert.NilError(t, err)
	assert.Assert(t, out == nil)
}
