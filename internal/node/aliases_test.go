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

package node

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/tree"
)

func normalize(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	assert.NilError(t, NormalizeAliases(&doc))
	return &doc
}

// findAlias returns true if any node in the subtree is an AliasNode.
func findAlias(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.AliasNode {
		return true
	}
	for _, c := range n.Content {
		if findAlias(c) {
			return true
		}
	}
	return false
}

// findMergeKey returns true if any MappingNode in the subtree still has a
// "<<" key (which NormalizeAliases is supposed to remove).
func findMergeKey(n *yaml.Node) bool {
	if n == nil {
		return false
	}
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "<<" {
				return true
			}
		}
	}
	for _, c := range n.Content {
		if findMergeKey(c) {
			return true
		}
	}
	return false
}

func decodeMap(t *testing.T, n *yaml.Node) map[string]any {
	t.Helper()
	var m map[string]any
	assert.NilError(t, n.Decode(&m))
	return m
}

func TestNormalizeAliasesUnfoldsSimpleAlias(t *testing.T) {
	src := `
defaults: &defaults
  image: nginx
  restart: always
services:
  web: *defaults
`
	root := normalize(t, src)
	assert.Assert(t, !findAlias(root), "no AliasNode should remain after NormalizeAliases")

	m := decodeMap(t, root)
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
	assert.Equal(t, web["restart"], "always")
}

func TestNormalizeAliasesFoldsMergeKeyWithSurroundingWins(t *testing.T) {
	src := `
defaults: &defaults
  image: nginx
  restart: always
services:
  web:
    <<: *defaults
    image: caddy
`
	root := normalize(t, src)
	assert.Assert(t, !findAlias(root))
	assert.Assert(t, !findMergeKey(root))

	m := decodeMap(t, root)
	web := m["services"].(map[string]any)["web"].(map[string]any)
	// Surrounding mapping wins over merge source.
	assert.Equal(t, web["image"], "caddy")
	assert.Equal(t, web["restart"], "always")
}

func TestNormalizeAliasesFoldsMergeKeySequence(t *testing.T) {
	// YAML 1.1 merge key with a sequence value: earlier entries win over
	// later ones; both lose to keys defined in the surrounding mapping.
	src := `
common: &common
  image: nginx
  ports: ["80:80"]
overrides: &overrides
  image: caddy
  restart: always
services:
  web:
    <<: [*common, *overrides]
`
	root := normalize(t, src)
	assert.Assert(t, !findAlias(root))
	assert.Assert(t, !findMergeKey(root))

	m := decodeMap(t, root)
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx", "first merge source wins")
	assert.Equal(t, web["restart"], "always")
	ports := web["ports"].([]any)
	assert.Equal(t, len(ports), 1)
	assert.Equal(t, ports[0], "80:80")
}

func TestNormalizeAliasesDeepCopiesSoMutationsAreIsolated(t *testing.T) {
	src := `
defaults: &defaults
  ports: ["80:80"]
services:
  web:
    <<: *defaults
  api:
    <<: *defaults
`
	root := normalize(t, src)

	// Mutate web.ports by appending to its yaml.Node Content directly.
	// If web and api shared the same Node, api would see the mutation.
	m := decodeMap(t, root)
	web := m["services"].(map[string]any)["web"].(map[string]any)
	api := m["services"].(map[string]any)["api"].(map[string]any)
	assert.DeepEqual(t, web["ports"], []any{"80:80"})
	assert.DeepEqual(t, api["ports"], []any{"80:80"})

	// Inspect the Content pointers to confirm divergence after the deep copy.
	var webPorts, apiPorts *yaml.Node
	for _, top := range root.Content[0].Content { // unwrap doc → root mapping
		// ignore the unwrap mechanics; instead walk to find both ports
		_ = top
	}
	_ = Walk(root, func(p tree.Path, n *yaml.Node) error {
		switch p.String() {
		case "services.web.ports":
			webPorts = n
		case "services.api.ports":
			apiPorts = n
		}
		return nil
	})
	assert.Assert(t, webPorts != nil && apiPorts != nil)
	assert.Assert(t, webPorts != apiPorts, "deep copy must produce distinct Node pointers")
}

func TestNormalizeAliasesPreservesLineForDiagnostics(t *testing.T) {
	src := `defaults: &defaults
  image: nginx
services:
  web: *defaults
`
	root := normalize(t, src)
	var imageLine int
	_ = Walk(root, func(p tree.Path, n *yaml.Node) error {
		if p.String() == "services.web.image" {
			imageLine = n.Line
		}
		return nil
	})
	// image: nginx is on line 2 of the source; the deep copy preserves it.
	assert.Equal(t, imageLine, 2)
}

func TestNormalizeAliasesRejectsCycle(t *testing.T) {
	src := `
a: &a
  loop: *a
`
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	err := NormalizeAliases(&doc)
	assert.ErrorContains(t, err, "cycle detected in alias chain")
}

func TestNormalizeAliasesHandlesAliasBomb(t *testing.T) {
	// Branching factor 3, depth 10: 3^10 = ~59k logical references; without
	// the `cleaned` cache the unfold would explode. With the cache, each
	// anchor is unfolded once.
	src := `
x-a: &a {k: v}
x-b: &b [*a, *a, *a]
x-c: &c [*b, *b, *b]
x-d: &d [*c, *c, *c]
x-e: &e [*d, *d, *d]
x-f: &f [*e, *e, *e]
x-g: &g [*f, *f, *f]
x-h: &h [*g, *g, *g]
x-i: &i [*h, *h, *h]
x-j: &j [*i, *i, *i]
x-k: &k [*j, *j, *j]
services:
  svc:
    image: alpine
`
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	assert.NilError(t, NormalizeAliases(&doc))
	assert.Assert(t, !findAlias(&doc))
}

func TestNormalizeAliasesHandlesNullSafely(t *testing.T) {
	// An empty top-level document must not panic.
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(""), &doc))
	assert.NilError(t, NormalizeAliases(&doc))
}

func TestNormalizeAliasesNestedAliasInsideAlias(t *testing.T) {
	src := `
inner: &inner
  k: v
outer: &outer
  ref: *inner
target: *outer
`
	root := normalize(t, src)
	assert.Assert(t, !findAlias(root))
	m := decodeMap(t, root)
	target := m["target"].(map[string]any)
	ref := target["ref"].(map[string]any)
	assert.Equal(t, ref["k"], "v")
}

func TestNormalizeAliasesPreservesAcrossMultipleCalls(t *testing.T) {
	src := `
common: &common {image: nginx}
services:
  web:
    <<: *common
`
	root := normalize(t, src)
	// A second call is a no-op (idempotent).
	assert.NilError(t, NormalizeAliases(root))
	m := decodeMap(t, root)
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
	// Sanity: no stray markers left in source.
	out, err := yaml.Marshal(root)
	assert.NilError(t, err)
	assert.Assert(t, !strings.Contains(string(out), "<<"), "no merge key in output")
}
