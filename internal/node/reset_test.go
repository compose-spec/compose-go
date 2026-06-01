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
	"fmt"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func resolveYAML(t *testing.T, src string) (*yaml.Node, []string, error) {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		return nil, nil, err
	}
	resolved, paths, err := ResolveResetOverride(&doc, 0)
	if err != nil {
		return nil, nil, err
	}
	strs := make([]string, len(paths))
	for i, p := range paths {
		strs[i] = p.String()
	}
	return resolved, strs, nil
}

func TestResolveResetTagRemovesNode(t *testing.T) {
	src := `
services:
  web:
    image: nginx
    command: !reset null
`
	resolved, paths, err := resolveYAML(t, src)
	assert.NilError(t, err)
	assert.DeepEqual(t, paths, []string{"services.web.command"})

	// Confirm command is no longer present in the resolved tree.
	out, err := yaml.Marshal(resolved)
	assert.NilError(t, err)
	assert.Assert(t, !strings.Contains(string(out), "command"), "command should be stripped from tree, got:\n%s", out)
}

func TestResolveOverrideTagKeepsNode(t *testing.T) {
	src := `
services:
  web:
    command: !override ["echo", "hi"]
`
	resolved, paths, err := resolveYAML(t, src)
	assert.NilError(t, err)
	assert.DeepEqual(t, paths, []string{"services.web.command"})

	out, err := yaml.Marshal(resolved)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(out), "command"), "command must survive !override, got:\n%s", out)
	assert.Assert(t, strings.Contains(string(out), "echo"), "command value preserved, got:\n%s", out)
}

func TestResolveNoTagsReturnsEmptyPaths(t *testing.T) {
	src := `
services:
  web:
    image: nginx
`
	_, paths, err := resolveYAML(t, src)
	assert.NilError(t, err)
	assert.Equal(t, len(paths), 0)
}

func TestResolveAliasCycleRejected(t *testing.T) {
	// A mapping that merges its own ancestor through `<<` creates an
	// alias cycle reachable via path containment, which resolveReset
	// detects. Pattern lifted from the loader-level TestResetCycle
	// "direct_self_reference_cycle" case.
	src := `
name: test
x-healthcheck: &healthcheck
  egress-service:
    <<: *healthcheck
`
	_, _, err := resolveYAML(t, src)
	assert.ErrorContains(t, err, "cycle detected")
}

func TestResolveMaxNodeVisitsExceeded(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("name: test\nentries:\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&sb, "  k%d: v\n", i)
	}
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(sb.String()), &doc))

	_, _, err := ResolveResetOverride(&doc, 50)
	assert.ErrorContains(t, err, "exceeds maximum node visit limit (50)")
}

func TestResolveSharedAnchorReplaysRelativePaths(t *testing.T) {
	// A shared anchor used by two services must record !reset at both call
	// sites, not just the first one. This is the regression covered by the
	// loader-level TestResetTagWithSharedAlias.
	src := `
x-base: &base
  command: !reset null

services:
  web: *base
  api: *base
`
	_, paths, err := resolveYAML(t, src)
	assert.NilError(t, err)
	// The anchor itself is at x-base.command; replayed paths are
	// services.web.command and services.api.command.
	assert.Assert(t, len(paths) >= 3, "expected at least 3 reset paths, got %v", paths)
	have := map[string]bool{}
	for _, p := range paths {
		have[p] = true
	}
	assert.Assert(t, have["services.web.command"], "services.web.command should be recorded: %v", paths)
	assert.Assert(t, have["services.api.command"], "services.api.command should be recorded: %v", paths)
}
