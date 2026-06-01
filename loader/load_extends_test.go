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
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v3/internal/node"
	"github.com/compose-spec/compose-go/v3/types"
)

func loadParentLayer(t *testing.T, dir, content string) (*node.Layer, *Options) {
	t.Helper()
	return buildParent(t, dir, types.Mapping{}, content)
}

func TestApplyExtendsToLayer_SameFile(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
services:
  base:
    image: nginx
    restart: always
  web:
    extends: base
    command: ["nginx", "-g", "daemon off;"]
`)
	assert.NilError(t, ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{}))

	var m map[string]any
	assert.NilError(t, parent.Node.Decode(&m))
	web := m["services"].(map[string]any)["web"].(map[string]any)
	// inherited from base
	assert.Equal(t, web["image"], "nginx")
	assert.Equal(t, web["restart"], "always")
	// own override survives
	assert.DeepEqual(t, web["command"], []any{"nginx", "-g", "daemon off;"})
	// extends key stripped from result
	_, hasExtends := web["extends"]
	assert.Assert(t, !hasExtends, "extends key must be removed after merge")
}

func TestApplyExtendsToLayer_LongFormService(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
services:
  base:
    image: nginx
  web:
    extends:
      service: base
    restart: always
`)
	assert.NilError(t, ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{}))

	var m map[string]any
	assert.NilError(t, parent.Node.Decode(&m))
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
	assert.Equal(t, web["restart"], "always")
}

func TestApplyExtendsToLayer_FromOtherFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `
services:
  base:
    image: nginx
    restart: always
`)
	parent, opts := loadParentLayer(t, dir, `
services:
  web:
    extends:
      file: base.yaml
      service: base
    command: ["echo", "ok"]
`)
	assert.NilError(t, ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{}))

	var m map[string]any
	assert.NilError(t, parent.Node.Decode(&m))
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
	assert.Equal(t, web["restart"], "always")
	assert.DeepEqual(t, web["command"], []any{"echo", "ok"})
}

func TestApplyExtendsToLayer_ChainedExtends(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
services:
  grandparent:
    image: nginx
    restart: always
  parent:
    extends: grandparent
    environment:
      LEVEL: parent
  child:
    extends: parent
    environment:
      OWN: child
`)
	assert.NilError(t, ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{}))

	var m map[string]any
	assert.NilError(t, parent.Node.Decode(&m))
	child := m["services"].(map[string]any)["child"].(map[string]any)
	assert.Equal(t, child["image"], "nginx")
	assert.Equal(t, child["restart"], "always")
	envSeq := child["environment"].([]any)
	// environment is merged as a sequence; both parent and child entries
	// must be present (EnforceUnicity would dedupe later if needed).
	have := map[string]bool{}
	for _, e := range envSeq {
		have[e.(string)] = true
	}
	assert.Assert(t, have["LEVEL=parent"])
	assert.Assert(t, have["OWN=child"])
}

func TestApplyExtendsToLayer_DetectsCycle(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
services:
  a:
    extends: b
  b:
    extends: a
`)
	err := ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{})
	assert.ErrorContains(t, err, "Circular reference")
}

func TestApplyExtendsToLayer_MissingServiceErrors(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
services:
  web:
    extends: missing
`)
	err := ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{})
	assert.ErrorContains(t, err, "service \"missing\" not found")
}

func TestApplyExtendsToLayer_NoServicesBlockIsNoop(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
networks:
  default:
    driver: bridge
`)
	assert.NilError(t, ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{}))
}

func TestApplyExtendsToLayer_NoExtendsLeavesServicesAlone(t *testing.T) {
	dir := t.TempDir()
	parent, opts := loadParentLayer(t, dir, `
services:
  web:
    image: nginx
`)
	assert.NilError(t, ApplyExtendsToLayer(context.TODO(), parent, opts, &cycleTracker{}))
	var m map[string]any
	assert.NilError(t, parent.Node.Decode(&m))
	web := m["services"].(map[string]any)["web"].(map[string]any)
	assert.Equal(t, web["image"], "nginx")
}
