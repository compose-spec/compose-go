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
	"errors"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/tree"
)

func parse(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func collectPaths(t *testing.T, root *yaml.Node) []string {
	t.Helper()
	var paths []string
	err := Walk(root, func(p tree.Path, _ *yaml.Node) error {
		paths = append(paths, p.String())
		return nil
	})
	assert.NilError(t, err)
	return paths
}

func TestWalkFlatMapping(t *testing.T) {
	root := parse(t, `
name: demo
version: "3"
`)
	got := collectPaths(t, root)
	assert.DeepEqual(t, got, []string{"", "name", "version"})
}

func TestWalkNestedMappingAndSequence(t *testing.T) {
	root := parse(t, `
services:
  web:
    image: nginx
    ports:
      - "80:80"
      - "443:443"
`)
	got := collectPaths(t, root)
	assert.DeepEqual(t, got, []string{
		"",
		"services",
		"services.web",
		"services.web.image",
		"services.web.ports",
		"services.web.ports.[]",
		"services.web.ports.[]",
	})
}

func TestWalkUnwrapsDocumentNode(t *testing.T) {
	root := parse(t, `key: value`)
	assert.Equal(t, root.Kind, yaml.DocumentNode)
	var rootKind yaml.Kind
	err := Walk(root, func(p tree.Path, n *yaml.Node) error {
		if p == "" {
			rootKind = n.Kind
		}
		return nil
	})
	assert.NilError(t, err)
	assert.Equal(t, rootKind, yaml.MappingNode)
}

func TestWalkFollowsAliasOnce(t *testing.T) {
	root := parse(t, `
defaults: &d
  image: nginx
  ports:
    - "80:80"
services:
  web: *d
`)
	var webImage *yaml.Node
	err := Walk(root, func(p tree.Path, n *yaml.Node) error {
		if p == "services.web.image" {
			webImage = n
		}
		return nil
	})
	assert.NilError(t, err)
	assert.Assert(t, webImage != nil, "alias target reachable via services.web.image")
	assert.Equal(t, webImage.Value, "nginx")
}

func TestWalkBreaksAliasCycle(t *testing.T) {
	// Construct an artificial cycle: a MappingNode whose only value is an
	// AliasNode pointing back to that mapping. The YAML library does not
	// allow expressing this in source, so we build it by hand.
	mapping := &yaml.Node{Kind: yaml.MappingNode}
	key := &yaml.Node{Kind: yaml.ScalarNode, Value: "self"}
	alias := &yaml.Node{Kind: yaml.AliasNode, Alias: mapping}
	mapping.Content = []*yaml.Node{key, alias}

	count := 0
	err := Walk(mapping, func(_ tree.Path, _ *yaml.Node) error {
		count++
		if count > 100 {
			return errors.New("walk did not terminate")
		}
		return nil
	})
	assert.NilError(t, err)
}

func TestWalkPropagatesError(t *testing.T) {
	root := parse(t, `
services:
  web:
    image: nginx
`)
	boom := errors.New("boom")
	err := Walk(root, func(p tree.Path, _ *yaml.Node) error {
		if p == "services.web.image" {
			return boom
		}
		return nil
	})
	assert.ErrorIs(t, err, boom)
}

func TestLayerOriginDefaultsToContext(t *testing.T) {
	root := parse(t, `key: value`)
	ctx := &SourceContext{File: "test.yaml", WorkingDir: "/work"}
	layer := NewLayer(root, ctx)

	var scalar *yaml.Node
	err := Walk(root, func(p tree.Path, n *yaml.Node) error {
		if p == "key" {
			scalar = n
		}
		return nil
	})
	assert.NilError(t, err)
	assert.Equal(t, layer.Origin(scalar), ctx)
}

func TestLayerSetOriginOverridesDefault(t *testing.T) {
	root := parse(t, `key: value`)
	defaultCtx := &SourceContext{File: "main.yaml"}
	otherCtx := &SourceContext{File: "included.yaml"}
	layer := NewLayer(root, defaultCtx)

	var scalar *yaml.Node
	_ = Walk(root, func(p tree.Path, n *yaml.Node) error {
		if p == "key" {
			scalar = n
		}
		return nil
	})
	layer.SetOrigin(scalar, otherCtx)

	assert.Equal(t, layer.Origin(scalar), otherCtx)

	other := &yaml.Node{Kind: yaml.ScalarNode, Value: "untracked"}
	assert.Equal(t, layer.Origin(other), defaultCtx)
}
