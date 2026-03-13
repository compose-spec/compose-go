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
	"encoding/json"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestFindKey(t *testing.T) {
	node := NewMapping(
		KeyValue{Key: "image", Value: NewScalar("nginx")},
		KeyValue{Key: "command", Value: NewScalar("echo hello")},
	)

	// Find existing key
	keyNode, valNode := FindKey(node, "image")
	assert.Assert(t, keyNode != nil)
	assert.Assert(t, valNode != nil)
	assert.Equal(t, valNode.Value, "nginx")

	// Find another existing key
	keyNode, valNode = FindKey(node, "command")
	assert.Assert(t, keyNode != nil)
	assert.Assert(t, valNode != nil)
	assert.Equal(t, valNode.Value, "echo hello")

	// Missing key returns nil
	keyNode, valNode = FindKey(node, "missing")
	assert.Assert(t, keyNode == nil)
	assert.Assert(t, valNode == nil)

	// Nil node returns nil
	keyNode, valNode = FindKey(nil, "image")
	assert.Assert(t, keyNode == nil)
	assert.Assert(t, valNode == nil)
}

func TestSetKey(t *testing.T) {
	node := NewMapping(
		KeyValue{Key: "image", Value: NewScalar("nginx")},
	)

	// Set a new key
	SetKey(node, "command", NewScalar("echo hello"))
	_, valNode := FindKey(node, "command")
	assert.Assert(t, valNode != nil)
	assert.Equal(t, valNode.Value, "echo hello")

	// Replace an existing key
	SetKey(node, "image", NewScalar("alpine"))
	_, valNode = FindKey(node, "image")
	assert.Assert(t, valNode != nil)
	assert.Equal(t, valNode.Value, "alpine")

	// Verify total content length (2 key-value pairs = 4 nodes)
	assert.Equal(t, len(node.Content), 4)
}

func TestDeleteKey(t *testing.T) {
	node := NewMapping(
		KeyValue{Key: "image", Value: NewScalar("nginx")},
		KeyValue{Key: "command", Value: NewScalar("echo hello")},
		KeyValue{Key: "ports", Value: NewScalar("8080")},
	)

	// Delete a key
	DeleteKey(node, "command")

	// Verify it's gone
	_, valNode := FindKey(node, "command")
	assert.Assert(t, valNode == nil)

	// Verify other keys still present
	_, valNode = FindKey(node, "image")
	assert.Assert(t, valNode != nil)
	assert.Equal(t, valNode.Value, "nginx")

	_, valNode = FindKey(node, "ports")
	assert.Assert(t, valNode != nil)
	assert.Equal(t, valNode.Value, "8080")

	// Content length should be 4 (2 remaining pairs)
	assert.Equal(t, len(node.Content), 4)
}

func TestNewScalar(t *testing.T) {
	n := NewScalar("hello")
	assert.Equal(t, n.Kind, yaml.ScalarNode)
	assert.Equal(t, n.Value, "hello")
	assert.Equal(t, n.Tag, "!!str")
}

func TestNewMapping(t *testing.T) {
	n := NewMapping(
		KeyValue{Key: "a", Value: NewScalar("1")},
		KeyValue{Key: "b", Value: NewScalar("2")},
	)
	assert.Equal(t, n.Kind, yaml.MappingNode)
	assert.Equal(t, len(n.Content), 4) // 2 key-value pairs
}

func TestNewSequence(t *testing.T) {
	n := NewSequence(NewScalar("a"), NewScalar("b"), NewScalar("c"))
	assert.Equal(t, n.Kind, yaml.SequenceNode)
	assert.Equal(t, len(n.Content), 3)
}

// testMergeParity verifies that MergeNodes produces the same result as MergeYaml.
func testMergeParity(t *testing.T, baseYAML, overrideYAML string) {
	t.Helper()

	// Parse as map[string]any
	var baseMap, overrideMap map[string]any
	assert.NilError(t, yaml.Unmarshal([]byte(baseYAML), &baseMap))
	assert.NilError(t, yaml.Unmarshal([]byte(overrideYAML), &overrideMap))

	// Parse as yaml.Node
	var baseNode, overrideNode yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(baseYAML), &baseNode))
	assert.NilError(t, yaml.Unmarshal([]byte(overrideYAML), &overrideNode))

	// Merge maps
	mapResult, err := MergeYaml(baseMap, overrideMap, tree.NewPath())
	assert.NilError(t, err)

	// Merge nodes - unwrap DocumentNode
	bn := &baseNode
	if bn.Kind == yaml.DocumentNode && len(bn.Content) > 0 {
		bn = bn.Content[0]
	}
	on := &overrideNode
	if on.Kind == yaml.DocumentNode && len(on.Content) > 0 {
		on = on.Content[0]
	}

	nodeResult, err := MergeNodes(bn, on, tree.NewPath())
	assert.NilError(t, err)

	// Decode node result to map
	var nodeMap map[string]any
	assert.NilError(t, nodeResult.Decode(&nodeMap))

	// Compare using JSON marshalling to normalize types
	mapJSON, _ := json.Marshal(mapResult)
	nodeJSON, _ := json.Marshal(nodeMap)
	assert.Equal(t, string(mapJSON), string(nodeJSON))
}

func TestMergeNodes_SimpleScalar(t *testing.T) {
	base := `
services:
  web:
    image: nginx
`
	override := `
services:
  web:
    image: alpine
`
	testMergeParity(t, base, override)
}

func TestMergeNodes_AddService(t *testing.T) {
	base := `
services:
  web:
    image: nginx
`
	override := `
services:
  db:
    image: postgres
`
	testMergeParity(t, base, override)
}

func TestMergeNodes_MergeLabels(t *testing.T) {
	base := `
services:
  web:
    image: nginx
    labels:
      foo: bar
`
	override := `
services:
  web:
    labels:
      baz: qux
`
	testMergeParity(t, base, override)
}

func TestMergeNodes_MergeDependsOn(t *testing.T) {
	base := `
services:
  web:
    image: nginx
    depends_on:
      - db
  db:
    image: postgres
`
	override := `
services:
  web:
    depends_on:
      - cache
  cache:
    image: redis
`
	testMergeParity(t, base, override)
}

func TestMergeNodes_MergeBuild(t *testing.T) {
	base := `
services:
  web:
    build: .
`
	override := `
services:
  web:
    build:
      dockerfile: Dockerfile.dev
`
	testMergeParity(t, base, override)
}

func TestMergeNodes_OverrideCommand(t *testing.T) {
	base := `
services:
  web:
    image: nginx
    command: ["nginx", "-g", "daemon off;"]
`
	override := `
services:
  web:
    command: ["echo", "hello"]
`
	testMergeParity(t, base, override)
}

func TestMergeNodes_MergeEnvironment(t *testing.T) {
	base := `
services:
  web:
    image: nginx
    environment:
      FOO: bar
`
	override := `
services:
  web:
    environment:
      BAZ: qux
`
	testMergeParity(t, base, override)
}
