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

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/tree"
)

// mergeNodeYAML parses two YAML strings, merges them through MergeNode and
// returns the result decoded back to a map[string]any so we can compare it
// against an expected YAML snippet with the same DeepEqual helper used by
// the v2 suite.
func mergeNodeYAML(t *testing.T, right, left string) map[string]any {
	t.Helper()
	r := parseNode(t, right)
	l := parseNode(t, left)
	merged, err := MergeNode(r, l, tree.NewPath())
	assert.NilError(t, err)
	var out map[string]any
	assert.NilError(t, merged.Decode(&out))
	return out
}

func parseNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func TestMergeNode_BasicOverride(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  test:
    image: foo
    scale: 1
`, `
services:
  test:
    image: bar
    scale: 2
`)
	assert.DeepEqual(t, got, unmarshal(t, `
services:
  test:
    image: bar
    scale: 2
`))
}

func TestMergeNode_MapAddsNewKey(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    image: nginx
`, `
services:
  web:
    image: nginx
    restart: always
`)
	web := got["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["restart"], "always")
}

func TestMergeNode_SequenceAppends(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    ports:
      - "80:80"
`, `
services:
  web:
    ports:
      - "443:443"
`)
	ports := got["services"].(map[string]any)["web"].(map[string]any)["ports"].([]any)
	assert.DeepEqual(t, ports, []any{"80:80", "443:443"})
}

func TestMergeNode_CommandIsOverridden(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    command: ["nginx", "-g", "daemon off;"]
`, `
services:
  web:
    command: ["caddy", "run"]
`)
	cmd := got["services"].(map[string]any)["web"].(map[string]any)["command"].([]any)
	assert.DeepEqual(t, cmd, []any{"caddy", "run"})
}

func TestMergeNode_BuildShortFormPromoted(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    build: ./base
`, `
services:
  web:
    build:
      dockerfile: Dockerfile.dev
`)
	build := got["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], "./base")
	assert.Equal(t, build["dockerfile"], "Dockerfile.dev")
}

func TestMergeNode_DependsOnListPromoted(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    depends_on:
      - db
`, `
services:
  web:
    depends_on:
      api:
        condition: service_healthy
`)
	deps := got["services"].(map[string]any)["web"].(map[string]any)["depends_on"].(map[string]any)
	db := deps["db"].(map[string]any)
	assert.Equal(t, db["condition"], "service_started")
	assert.Equal(t, db["required"], true)
	api := deps["api"].(map[string]any)
	assert.Equal(t, api["condition"], "service_healthy")
}

func TestMergeNode_NetworksListPromoted(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    networks:
      - frontend
`, `
services:
  web:
    networks:
      backend:
        priority: 5
`)
	nets := got["services"].(map[string]any)["web"].(map[string]any)["networks"].(map[string]any)
	_, hasFront := nets["frontend"]
	assert.Assert(t, hasFront, "frontend network preserved")
	back := nets["backend"].(map[string]any)
	assert.Equal(t, back["priority"], 5)
}

func TestMergeNode_ExtraHostsDedupes(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    extra_hosts:
      - "host1:1.2.3.4"
`, `
services:
  web:
    extra_hosts:
      - "host1:1.2.3.4"
      - "host2:5.6.7.8"
`)
	hosts := got["services"].(map[string]any)["web"].(map[string]any)["extra_hosts"].([]any)
	assert.DeepEqual(t, hosts, []any{"host1:1.2.3.4", "host2:5.6.7.8"})
}

func TestMergeNode_LoggingSameDriverMergesOptions(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    logging:
      driver: json-file
      options:
        max-size: "10m"
`, `
services:
  web:
    logging:
      driver: json-file
      options:
        max-file: "3"
`)
	logging := got["services"].(map[string]any)["web"].(map[string]any)["logging"].(map[string]any)
	opts := logging["options"].(map[string]any)
	assert.Equal(t, opts["max-size"], "10m")
	assert.Equal(t, opts["max-file"], "3")
}

func TestMergeNode_LoggingDifferentDriverReplaces(t *testing.T) {
	got := mergeNodeYAML(t, `
services:
  web:
    logging:
      driver: json-file
      options:
        max-size: "10m"
`, `
services:
  web:
    logging:
      driver: syslog
      options:
        tag: web
`)
	logging := got["services"].(map[string]any)["web"].(map[string]any)["logging"].(map[string]any)
	assert.Equal(t, logging["driver"], "syslog")
	opts := logging["options"].(map[string]any)
	_, hasMaxSize := opts["max-size"]
	assert.Assert(t, !hasMaxSize, "max-size from json-file driver must not leak")
	assert.Equal(t, opts["tag"], "web")
}

func TestMergeNode_EnvironmentListMerged(t *testing.T) {
	// environment uses mergeToSequence: both lists are concatenated, no
	// deduplication at merge time (EnforceUnicity handles that downstream).
	got := mergeNodeYAML(t, `
services:
  web:
    environment:
      FOO: "1"
      BAR: "2"
`, `
services:
  web:
    environment:
      - "BAZ=3"
`)
	env := got["services"].(map[string]any)["web"].(map[string]any)["environment"].([]any)
	// Mapping is sorted; sequence appends after.
	assert.Equal(t, len(env), 3)
	// Sort-based equality check
	have := map[string]bool{}
	for _, e := range env {
		have[e.(string)] = true
	}
	assert.Assert(t, have["FOO=1"])
	assert.Assert(t, have["BAR=2"])
	assert.Assert(t, have["BAZ=3"])
}

func TestMergeNode_IPAMSubnetMatching(t *testing.T) {
	got := mergeNodeYAML(t, `
networks:
  app:
    ipam:
      config:
        - subnet: 10.0.0.0/24
          gateway: 10.0.0.1
        - subnet: 10.1.0.0/24
`, `
networks:
  app:
    ipam:
      config:
        - subnet: 10.0.0.0/24
          gateway: 10.0.0.254
        - subnet: 10.2.0.0/24
`)
	conf := got["networks"].(map[string]any)["app"].(map[string]any)["ipam"].(map[string]any)["config"].([]any)
	bySubnet := map[string]map[string]any{}
	for _, e := range conf {
		m := e.(map[string]any)
		bySubnet[m["subnet"].(string)] = m
	}
	assert.Equal(t, bySubnet["10.0.0.0/24"]["gateway"], "10.0.0.254", "matching subnet: left wins")
	_, ok := bySubnet["10.1.0.0/24"]
	assert.Assert(t, ok, "10.1.0.0/24 preserved from right")
	_, ok = bySubnet["10.2.0.0/24"]
	assert.Assert(t, ok, "10.2.0.0/24 appended from left")
}

func TestMergeNode_ScalarOverride(t *testing.T) {
	got := mergeNodeYAML(t, `name: a`, `name: b`)
	assert.Equal(t, got["name"], "b")
}

func TestMergeNode_NilLeftReturnsRight(t *testing.T) {
	right := parseNode(t, `services: {web: {image: nginx}}`)
	merged, err := MergeNode(right, nil, tree.NewPath())
	assert.NilError(t, err)
	var out map[string]any
	assert.NilError(t, merged.Decode(&out))
	assert.Equal(t, out["services"].(map[string]any)["web"].(map[string]any)["image"], "nginx")
}

func TestMergeNode_PreservesLineNumbers(t *testing.T) {
	right := parseNode(t, "services:\n  web:\n    image: nginx\n")
	left := parseNode(t, "services:\n  web:\n    restart: always\n")
	merged, err := MergeNode(right, left, tree.NewPath())
	assert.NilError(t, err)

	// Find the image scalar; it must keep its line 3 from right.
	imageLine := 0
	restartLine := 0
	var visit func(n *yaml.Node)
	visit = func(n *yaml.Node) {
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == "image" {
					imageLine = n.Content[i+1].Line
				}
				if n.Content[i].Value == "restart" {
					restartLine = n.Content[i+1].Line
				}
				visit(n.Content[i+1])
			}
			return
		}
		for _, c := range n.Content {
			visit(c)
		}
	}
	visit(merged)
	assert.Equal(t, imageLine, 3, "right's image scalar retains its source line")
	assert.Equal(t, restartLine, 3, "left's restart scalar retains its own source line")
}
