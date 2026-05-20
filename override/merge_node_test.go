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

	"github.com/compose-spec/compose-go/v3/tree"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func parseYaml(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var n yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &n))
	return &n
}

func renderYaml(t *testing.T, n *yaml.Node) string {
	t.Helper()
	out, err := yaml.Marshal(n)
	assert.NilError(t, err)
	return string(out)
}

func TestMergeNodes_ScalarOverride(t *testing.T) {
	base := parseYaml(t, "services:\n  app:\n    image: alpine\n")
	over := parseYaml(t, "services:\n  app:\n    image: nginx\n")
	merged, err := MergeNodes(base.Content[0], over.Content[0], tree.NewPath())
	assert.NilError(t, err)
	rendered := renderYaml(t, merged)
	assert.Assert(t, contains(rendered, "image: nginx"))
}

func TestMergeNodes_MappingMerge(t *testing.T) {
	base := parseYaml(t, "services:\n  app:\n    image: alpine\n    command: a\n")
	over := parseYaml(t, "services:\n  app:\n    command: b\n    user: root\n")
	merged, err := MergeNodes(base.Content[0], over.Content[0], tree.NewPath())
	assert.NilError(t, err)
	rendered := renderYaml(t, merged)
	assert.Assert(t, contains(rendered, "image: alpine"))
	assert.Assert(t, contains(rendered, "command: b"))
	assert.Assert(t, contains(rendered, "user: root"))
}

func TestMergeNodes_PreservesLeafIdentity(t *testing.T) {
	base := parseYaml(t, "services:\n  app:\n    image: alpine\n    command: a\n")
	over := parseYaml(t, "services:\n  app:\n    command: b\n")

	// Capture the original image scalar pointer (the leaf that override never
	// touches).
	_, baseServices := FindKey(base.Content[0], "services")
	_, baseApp := FindKey(baseServices, "app")
	_, baseImage := FindKey(baseApp, "image")
	assert.Assert(t, baseImage != nil)

	merged, err := MergeNodes(base.Content[0], over.Content[0], tree.NewPath())
	assert.NilError(t, err)

	_, mergedServices := FindKey(merged, "services")
	_, mergedApp := FindKey(mergedServices, "app")
	_, mergedImage := FindKey(mergedApp, "image")
	assert.Equal(t, mergedImage, baseImage)
}

func TestMergeNodes_ResetRemovesKey(t *testing.T) {
	base := parseYaml(t, "services:\n  app:\n    image: alpine\n    command: a\n")
	over := parseYaml(t, "services:\n  app:\n    command: !reset\n")
	merged, err := MergeNodes(base.Content[0], over.Content[0], tree.NewPath())
	assert.NilError(t, err)
	rendered := renderYaml(t, merged)
	assert.Assert(t, contains(rendered, "image: alpine"))
	assert.Assert(t, !contains(rendered, "command:"))
}

func TestMergeNodes_OverrideTagReplacesSubtree(t *testing.T) {
	base := parseYaml(t, "services:\n  app:\n    environment:\n      - FOO=1\n")
	over := parseYaml(t, "services:\n  app:\n    environment: !override\n      - BAR=2\n")
	merged, err := MergeNodes(base.Content[0], over.Content[0], tree.NewPath())
	assert.NilError(t, err)
	rendered := renderYaml(t, merged)
	assert.Assert(t, !contains(rendered, "FOO=1"))
	assert.Assert(t, contains(rendered, "BAR=2"))
}

func TestMergeNodes_EnvironmentMappingAndSequence(t *testing.T) {
	base := parseYaml(t, "services:\n  app:\n    environment:\n      FOO: 1\n")
	over := parseYaml(t, "services:\n  app:\n    environment:\n      - BAR=2\n")
	merged, err := MergeNodes(base.Content[0], over.Content[0], tree.NewPath())
	assert.NilError(t, err)
	rendered := renderYaml(t, merged)
	assert.Assert(t, contains(rendered, "FOO=1"))
	assert.Assert(t, contains(rendered, "BAR=2"))
}

func TestEnforceUnicityNode_DedupesEnvironment(t *testing.T) {
	doc := parseYaml(t, "services:\n  app:\n    environment:\n      - FOO=1\n      - FOO=2\n      - BAR=3\n")
	EnforceUnicityNode(doc.Content[0], tree.NewPath())
	rendered := renderYaml(t, doc)
	// FOO=2 must keep, FOO=1 must be dropped (last wins).
	assert.Assert(t, !contains(rendered, "- FOO=1"))
	assert.Assert(t, contains(rendered, "- FOO=2"))
	assert.Assert(t, contains(rendered, "- BAR=3"))
}

func TestStripResetTags_RemovesUnmatchedResets(t *testing.T) {
	doc := parseYaml(t, "services:\n  app:\n    command: !reset\n")
	StripResetTags(doc)
	rendered := renderYaml(t, doc)
	assert.Assert(t, !contains(rendered, "command:"))
}

func TestFindKey_SetKey_DeleteKey(t *testing.T) {
	doc := parseYaml(t, "a: 1\nb: 2\nc: 3\n")
	mapping := doc.Content[0]
	k, v := FindKey(mapping, "b")
	assert.Assert(t, k != nil)
	assert.Equal(t, v.Value, "2")

	SetKey(mapping, "b", NewScalar("changed"))
	_, v2 := FindKey(mapping, "b")
	assert.Equal(t, v2.Value, "changed")

	SetKey(mapping, "d", NewScalar("4"))
	_, v3 := FindKey(mapping, "d")
	assert.Equal(t, v3.Value, "4")

	DeleteKey(mapping, "a")
	k4, _ := FindKey(mapping, "a")
	assert.Assert(t, k4 == nil)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
