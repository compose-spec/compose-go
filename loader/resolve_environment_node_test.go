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

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/tree"
	"github.com/compose-spec/compose-go/v3/types"
)

func TestResolveEnvironmentNode_BareKeyResolvedAgainstScalarContext(t *testing.T) {
	src := `
services:
  web:
    environment:
      - FOO
      - BAR=2
  api:
    environment:
      - FOO
`
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))

	// Build distinct SourceContexts and attach them to the right scalars
	// so the resolver can demonstrate per-scalar lookup.
	parentCtx := &node.SourceContext{Environment: types.Mapping{"FOO": "parent-value"}}
	includeCtx := &node.SourceContext{Environment: types.Mapping{"FOO": "include-value"}}

	origins := map[*yaml.Node]*node.SourceContext{}
	var webFOO, apiFOO *yaml.Node
	_ = node.Walk(&doc, func(p tree.Path, n *yaml.Node) error {
		switch p.String() {
		case "services.web.environment.[]":
			if n.Value == "FOO" {
				webFOO = n
			}
		case "services.api.environment.[]":
			if n.Value == "FOO" {
				apiFOO = n
			}
		}
		return nil
	})
	assert.Assert(t, webFOO != nil && apiFOO != nil)
	origins[webFOO] = parentCtx
	origins[apiFOO] = includeCtx

	ResolveEnvironmentNode(&doc, origins)

	var m map[string]any
	assert.NilError(t, doc.Decode(&m))
	web := m["services"].(map[string]any)["web"].(map[string]any)["environment"].([]any)
	api := m["services"].(map[string]any)["api"].(map[string]any)["environment"].([]any)
	assert.Equal(t, web[0], "FOO=parent-value")
	assert.Equal(t, web[1], "BAR=2", "key=value entries are left alone")
	assert.Equal(t, api[0], "FOO=include-value")
}

func TestResolveEnvironmentNode_MissingVariableLeftAlone(t *testing.T) {
	src := `
services:
  web:
    environment:
      - UNKNOWN
`
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))

	origins := map[*yaml.Node]*node.SourceContext{}
	var bare *yaml.Node
	_ = node.Walk(&doc, func(p tree.Path, n *yaml.Node) error {
		if p.String() == "services.web.environment.[]" && n.Value == "UNKNOWN" {
			bare = n
		}
		return nil
	})
	assert.Assert(t, bare != nil)
	origins[bare] = &node.SourceContext{Environment: types.Mapping{"OTHER": "value"}}

	ResolveEnvironmentNode(&doc, origins)

	var m map[string]any
	assert.NilError(t, doc.Decode(&m))
	env := m["services"].(map[string]any)["web"].(map[string]any)["environment"].([]any)
	assert.Equal(t, env[0], "UNKNOWN", "unresolved keys stay bare")
}

func TestResolveEnvironmentNode_NoServicesNoOp(t *testing.T) {
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte("networks: {default: {}}"), &doc))
	// Must not panic on configs without services.
	ResolveEnvironmentNode(&doc, map[*yaml.Node]*node.SourceContext{})
}
