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

package paths

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// testProjectDir is the absolute project root used by the in-package
// resolver tests. It is built from a single-letter component so the
// gocritic filepathJoin checker does not flag literal path separators in
// filepath.Join calls below.
var testProjectDir = filepath.Join(string(filepath.Separator), "project")

func parse(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	assert.NilError(t, yaml.Unmarshal([]byte(src), &doc))
	return &doc
}

func resolveYAML(t *testing.T, src string) map[string]any {
	t.Helper()
	root := parse(t, src)
	assert.NilError(t, ResolveRelativePathsNode(root, NodeResolverOptions{WorkingDir: testProjectDir}))
	var m map[string]any
	assert.NilError(t, root.Decode(&m))
	return m
}

func TestResolveRelativePathsNode_BuildContextResolved(t *testing.T) {
	got := resolveYAML(t, `
services:
  web:
    build:
      context: ./app
`)
	build := got["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], filepath.Join(testProjectDir, "app"))
}

func TestResolveRelativePathsNode_BuildContextUrlUntouched(t *testing.T) {
	got := resolveYAML(t, `
services:
  web:
    build:
      context: https://github.com/example/repo.git
`)
	build := got["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], "https://github.com/example/repo.git")
}

func TestResolveRelativePathsNode_EnvFileLongFormResolved(t *testing.T) {
	got := resolveYAML(t, `
services:
  web:
    env_file:
      - path: ./.env
        required: true
`)
	env := got["services"].(map[string]any)["web"].(map[string]any)["env_file"].([]any)
	first := env[0].(map[string]any)
	assert.Equal(t, first["path"], filepath.Join(testProjectDir, ".env"))
}

func TestResolveRelativePathsNode_VolumesLongFormBindResolved(t *testing.T) {
	got := resolveYAML(t, `
services:
  web:
    volumes:
      - type: bind
        source: ./data
        target: /data
`)
	vols := got["services"].(map[string]any)["web"].(map[string]any)["volumes"].([]any)
	mount := vols[0].(map[string]any)
	expected := filepath.Join(testProjectDir, "data")
	assert.Equal(t, mount["source"], expected)
}

func TestResolveRelativePathsNode_VolumesShortFormUnchanged(t *testing.T) {
	got := resolveYAML(t, `
services:
  web:
    volumes:
      - "./data:/data"
`)
	vols := got["services"].(map[string]any)["web"].(map[string]any)["volumes"].([]any)
	// Short form: left untouched by the Node-level resolver.
	assert.Equal(t, vols[0], "./data:/data")
}

func TestResolveRelativePathsNode_AbsolutePathUnchanged(t *testing.T) {
	// t.TempDir returns an absolute path appropriate for the host OS
	// (`/var/...` on Unix, `C:\Users\...\Temp\...` on Windows), so the
	// test exercises filepath.IsAbs on whichever platform CI runs on.
	absoluteCtx := t.TempDir()
	src := "services:\n  web:\n    build:\n      context: " + strconv.Quote(absoluteCtx) + "\n"
	got := resolveYAML(t, src)
	build := got["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)
	assert.Equal(t, build["context"], absoluteCtx)
}

// TestResolveRelativePathsNode_PerScalarWorkingDir is the key v3 behavior:
// two relative paths in the same merged tree resolve against different
// working directories depending on the SourceContext attached to each.
func TestResolveRelativePathsNode_PerScalarWorkingDir(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "project-root")
	includeDir := filepath.Join(string(filepath.Separator), "include-dir")
	root := parse(t, `
services:
  web:
    build:
      context: ./from-root
  api:
    build:
      context: ./from-include
`)
	var webContext, apiContext *yaml.Node
	var walk func(n *yaml.Node, parentKeys []string)
	walk = func(n *yaml.Node, parentKeys []string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, parentKeys)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				walk(n.Content[i+1], append(parentKeys, n.Content[i].Value))
			}
		case yaml.ScalarNode:
			if len(parentKeys) >= 3 && parentKeys[len(parentKeys)-1] == "context" {
				switch parentKeys[len(parentKeys)-3] {
				case "web":
					webContext = n
				case "api":
					apiContext = n
				}
			}
		}
	}
	walk(root, nil)
	assert.Assert(t, webContext != nil && apiContext != nil)

	err := ResolveRelativePathsNode(root, NodeResolverOptions{
		WorkingDirFor: func(n *yaml.Node) string {
			if n == apiContext {
				return includeDir
			}
			return rootDir
		},
	})
	assert.NilError(t, err)

	var m map[string]any
	assert.NilError(t, root.Decode(&m))
	assert.Equal(t,
		m["services"].(map[string]any)["web"].(map[string]any)["build"].(map[string]any)["context"],
		filepath.Join(rootDir, "from-root"))
	assert.Equal(t,
		m["services"].(map[string]any)["api"].(map[string]any)["build"].(map[string]any)["context"],
		filepath.Join(includeDir, "from-include"))
}

// TestResolveRelativePathsNode_IncludeNotResolvedHere documents that
// `include` paths are intentionally not resolved by ResolveRelativePathsNode
// (mirroring v2): include path resolution is part of collectIncludeLayers /
// ApplyInclude which knows about ResourceLoaders and project_directory
// redefinition. The patterns kept under "include.*" in the resolver map are
// inert (they never match the actual `include.[].path` walk path) but are
// preserved for v2 parity.
func TestResolveRelativePathsNode_IncludeNotResolvedHere(t *testing.T) {
	got := resolveYAML(t, `
include:
  - path:
      - ./a.yaml
      - ./b.yaml
    project_directory: ./sub
`)
	incl := got["include"].([]any)[0].(map[string]any)
	paths := incl["path"].([]any)
	assert.Equal(t, paths[0], "./a.yaml")
	assert.Equal(t, paths[1], "./b.yaml")
	assert.Equal(t, incl["project_directory"], "./sub")
}

func TestResolveRelativePathsNode_RemoteExtendsUntouched(t *testing.T) {
	root := parse(t, `
services:
  web:
    extends:
      file: oci://registry/example:tag
      service: base
`)
	err := ResolveRelativePathsNode(root, NodeResolverOptions{
		WorkingDir: testProjectDir,
		Remotes: []RemoteResource{
			func(p string) bool { return strings.HasPrefix(p, "oci://") },
		},
	})
	assert.NilError(t, err)
	var m map[string]any
	assert.NilError(t, root.Decode(&m))
	ext := m["services"].(map[string]any)["web"].(map[string]any)["extends"].(map[string]any)
	assert.Equal(t, ext["file"], "oci://registry/example:tag")
}
