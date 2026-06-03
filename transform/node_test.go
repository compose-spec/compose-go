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

package transform

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func parseNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func canonicalize(t *testing.T, src string) map[string]any {
	t.Helper()
	root := parseNode(t, src)
	out, err := CanonicalNode(root, false)
	assert.NilError(t, err)
	var m map[string]any
	assert.NilError(t, out.Decode(&m))
	return m
}

func TestCanonicalNode_PortsShortFormExpanded(t *testing.T) {
	got := canonicalize(t, `
services:
  web:
    ports:
      - "8080:80"
`)
	ports := got["services"].(map[string]any)["web"].(map[string]any)["ports"].([]any)
	assert.Equal(t, len(ports), 1)
	long := ports[0].(map[string]any)
	assert.Equal(t, long["target"], 80)
	assert.Equal(t, long["published"], "8080")
}

func TestCanonicalNode_BuildShortFormExpanded(t *testing.T) {
	got := canonicalize(t, `
services:
  web:
    build: ./app
`)
	build := got["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], "./app")
}

func TestCanonicalNode_DependsOnListExpanded(t *testing.T) {
	got := canonicalize(t, `
services:
  web:
    depends_on:
      - db
      - cache
`)
	deps := got["services"].(map[string]any)["web"].(map[string]any)["depends_on"].(map[string]any)
	db := deps["db"].(map[string]any)
	assert.Equal(t, db["condition"], "service_started")
}

func TestCanonicalNode_NetworksListExpanded(t *testing.T) {
	got := canonicalize(t, `
services:
  web:
    networks:
      - frontend
      - backend
`)
	nets := got["services"].(map[string]any)["web"].(map[string]any)["networks"].(map[string]any)
	_, ok := nets["frontend"]
	assert.Assert(t, ok)
	_, ok = nets["backend"]
	assert.Assert(t, ok)
}

func TestCanonicalNode_EnvFileShortFormExpanded(t *testing.T) {
	got := canonicalize(t, `
services:
  web:
    env_file: .env
`)
	env := got["services"].(map[string]any)["web"].(map[string]any)["env_file"].([]any)
	first := env[0].(map[string]any)
	assert.Equal(t, first["path"], ".env")
}

func TestCanonicalNode_DocumentNodeUnwrapped(t *testing.T) {
	// Passing a DocumentNode root must not panic and must return a tree
	// whose decoded shape matches the expected canonical form.
	root := parseNode(t, "services:\n  web:\n    build: .\n")
	assert.Equal(t, root.Kind, yaml.DocumentNode)
	out, err := CanonicalNode(root, false)
	assert.NilError(t, err)
	var m map[string]any
	assert.NilError(t, out.Decode(&m))
	build := m["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], ".")
}

func TestCanonicalNode_NilSafelyHandled(t *testing.T) {
	out, err := CanonicalNode(nil, false)
	assert.NilError(t, err)
	assert.Assert(t, out == nil)
}
