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

package interpolation

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/tree"
)

func parseNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func decode(t *testing.T, n *yaml.Node) map[string]any {
	t.Helper()
	var m map[string]any
	assert.NilError(t, n.Decode(&m))
	return m
}

func mappingLookup(m map[string]string) LookupValue {
	return func(key string) (string, bool) { v, ok := m[key]; return v, ok }
}

func TestInterpolateNode_BasicSubstitution(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    image: nginx:${TAG}
    ports:
      - "${HOST_PORT}:80"
`)
	err := InterpolateNode(root, NodeOptions{
		LookupValue: mappingLookup(map[string]string{
			"TAG":       "1.2.3",
			"HOST_PORT": "8080",
		}),
	})
	assert.NilError(t, err)
	m := decode(t, root)
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx:1.2.3")
	assert.DeepEqual(t, web["ports"], []any{"8080:80"})
}

func TestInterpolateNode_LazyPerScalarLookup(t *testing.T) {
	// The same ${TAG} variable resolves differently depending on which scalar
	// we are interpolating: scalar A uses lookupA, scalar B uses lookupB.
	// This is the lazy-interpolation pattern the v3 include / extends paths
	// rely on to honor per-Layer SourceContext.
	root := parseNode(t, `
services:
  web:
    image: nginx:${TAG}
  api:
    image: caddy:${TAG}
`)

	lookupForWeb := mappingLookup(map[string]string{"TAG": "from-web"})
	lookupForAPI := mappingLookup(map[string]string{"TAG": "from-api"})

	// Pre-locate the two scalars by walking the tree (starting from the
	// inner mapping, after unwrapping the DocumentNode).
	var webImage, apiImage *yaml.Node
	var walk func(*yaml.Node, string)
	walk = func(n *yaml.Node, parentKey string) {
		if n.Kind == yaml.DocumentNode {
			for _, c := range n.Content {
				walk(c, parentKey)
			}
			return
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				walk(n.Content[i+1], n.Content[i].Value)
			}
		}
		if n.Kind == yaml.ScalarNode && parentKey == "image" {
			switch n.Value {
			case "nginx:${TAG}":
				webImage = n
			case "caddy:${TAG}":
				apiImage = n
			}
		}
	}
	walk(root, "")
	assert.Assert(t, webImage != nil && apiImage != nil)

	err := InterpolateNode(root, NodeOptions{
		LookupValueFor: func(n *yaml.Node) LookupValue {
			if n == apiImage {
				return lookupForAPI
			}
			return lookupForWeb
		},
	})
	assert.NilError(t, err)

	m := decode(t, root)
	assert.Equal(t, m["services"].(map[string]any)["web"].(map[string]any)["image"], "nginx:from-web")
	assert.Equal(t, m["services"].(map[string]any)["api"].(map[string]any)["image"], "caddy:from-api")
}

func TestInterpolateNode_TagApplied(t *testing.T) {
	root := parseNode(t, `
services:
  web:
    ports:
      - target: "${PORT}"
        protocol: tcp
`)
	err := InterpolateNode(root, NodeOptions{
		LookupValue: mappingLookup(map[string]string{"PORT": "80"}),
		Tags: map[tree.Path]string{
			"services.*.ports.[].target": "!!int",
		},
	})
	assert.NilError(t, err)

	// After interpolation + tag rewrite, the scalar Value is "80" and the
	// Tag is "!!int", so decoding to a struct with an int field succeeds
	// natively.
	type Port struct {
		Target   int    `yaml:"target"`
		Protocol string `yaml:"protocol"`
	}
	type WebService struct {
		Ports []Port `yaml:"ports"`
	}
	type ServicesBlock struct {
		Web WebService `yaml:"web"`
	}
	type Config struct {
		Services ServicesBlock `yaml:"services"`
	}
	var c Config
	assert.NilError(t, root.Decode(&c))
	assert.Equal(t, c.Services.Web.Ports[0].Target, 80)
}

func TestInterpolateNode_MissingVariableLeavesScalar(t *testing.T) {
	// template.Substitute without a strict mode leaves unmatched variables
	// as empty string by default; the same behavior is preserved here.
	root := parseNode(t, `
key: value-${MISSING}
`)
	err := InterpolateNode(root, NodeOptions{
		LookupValue: mappingLookup(map[string]string{}),
	})
	assert.NilError(t, err)
	m := decode(t, root)
	assert.Equal(t, m["key"], "value-")
}

func TestInterpolateNode_NullScalarSkipped(t *testing.T) {
	root := parseNode(t, `
key: ~
other: ${VAL}
`)
	err := InterpolateNode(root, NodeOptions{
		LookupValue: mappingLookup(map[string]string{"VAL": "hello"}),
	})
	assert.NilError(t, err)
	m := decode(t, root)
	assert.Assert(t, m["key"] == nil)
	assert.Equal(t, m["other"], "hello")
}

func TestInterpolateNode_PreservesStyle(t *testing.T) {
	// A double-quoted scalar must stay double-quoted in the marshaled output.
	root := parseNode(t, `key: "value-${VAR}"`)
	err := InterpolateNode(root, NodeOptions{
		LookupValue: mappingLookup(map[string]string{"VAR": "x"}),
	})
	assert.NilError(t, err)
	out, err := yaml.Marshal(root)
	assert.NilError(t, err)
	assert.Equal(t, string(out), "key: \"value-x\"\n")
}

func TestInterpolateNode_NoLookupReturnsError(t *testing.T) {
	root := parseNode(t, `key: value`)
	err := InterpolateNode(root, NodeOptions{})
	assert.ErrorContains(t, err, "LookupValueFor or LookupValue must be set")
}
