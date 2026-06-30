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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestLoadIncludeExtendsCombined(t *testing.T) {
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: "testdata/combined",
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "testdata/combined/compose.yaml",
			},
		},
	}, withProjectName("test-load-combined", true))
	assert.NilError(t, err)
}

func TestLoadWithMultipleInclude(t *testing.T) {
	// include same service twice should not trigger an error
	details := buildConfigDetails(`
name: 'test-multi-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env
  - path: ./testdata/compose-include.yaml

services:
  foo:
    image: busybox
    depends_on:
      - imported
`, map[string]string{"SOURCE": "override"})

	p, err := LoadWithContext(context.TODO(), details, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	imported, err := p.GetService("imported")
	assert.NilError(t, err)
	assert.Equal(t, imported.ContainerName, "override")
}

func TestLoadWithMultipleIncludeConflict(t *testing.T) {
	// include 2 different services with same name should trigger an error
	details := buildConfigDetails(`
name: 'test-multi-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env
  - path: ./testdata/compose-include.yaml
    env_file: ./testdata/subdir/extra.env


services:
  bar:
    image: busybox
    environment: !override
      - ZOT=QIX
`, map[string]string{"SOURCE": "override"})
	p, err := LoadWithContext(context.TODO(), details, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["bar"], types.ServiceConfig{
		Name:  "bar",
		Image: "busybox",
		Environment: types.MappingWithEquals{
			"ZOT": strPtr("QIX"),
		},
	})
}

func TestIncludeRelative(t *testing.T) {
	wd, err := filepath.Abs(filepath.Join("testdata", "include"))
	assert.NilError(t, err)
	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join("testdata", "include", "compose.yaml"),
			},
		},
		WorkingDir: wd,
	}, func(options *Options) {
		options.projectName = "test-include-relative"
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	included := p.Services["included"]
	assert.Equal(t, included.Build.Context, ".")
	assert.Equal(t, included.Volumes[0].Source, ".")
}

func TestLoadWithIncludeEnv(t *testing.T) {
	fileName := "compose.yml"
	tmpdir := t.TempDir()
	// file in root
	yaml := `
include:
  - path:
    - ./module/compose.yml
    env_file:
      - ./custom.env
services:
  a:
    image: alpine
    environment:
      - VAR_NAME`
	createFile(t, tmpdir, `VAR_NAME=value`, "custom.env")
	path := createFile(t, tmpdir, yaml, fileName)
	// file in /module
	yaml = `
services:
  b:
    image: alpine
    environment:
      - VAR_NAME
  c:
    image: alpine
    environment:
      - VAR_NAME`
	createFileSubDir(t, tmpdir, "module", yaml, fileName)

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: path,
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.SetProjectName("project", true)
	})
	assert.NilError(t, err)
	a := p.Services["a"]
	// make sure VAR_NAME is only accessible in include context
	assert.Check(t, a.Environment["VAR_NAME"] == nil, "VAR_NAME should not be defined in environment")
	b := p.Services["b"]
	assert.Check(t, b.Environment["VAR_NAME"] != nil, "VAR_NAME is not defined in environment")
	assert.Equal(t, *b.Environment["VAR_NAME"], "value")
	c := p.Services["c"]
	assert.Check(t, c.Environment["VAR_NAME"] != nil, "VAR_NAME is not defined in environment")
	assert.Equal(t, *c.Environment["VAR_NAME"], "value")
}

func TestIncludeWithProjectDirectory(t *testing.T) {
	var envs map[string]string
	if runtime.GOOS == "windows" {
		envs = map[string]string{"COMPOSE_CONVERT_WINDOWS_PATHS": "1"}
	}
	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  "testdata/include",
		Environment: envs,
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "testdata/include/project-directory.yaml",
			},
		},
	}, withProjectName("test-load-project-directory", true))
	assert.NilError(t, err)
	assert.Equal(t, filepath.ToSlash(p.Services["service"].Build.Context), "testdata/subdir")
	assert.Equal(t, filepath.ToSlash(p.Services["service"].Volumes[0].Source), "testdata/subdir/compose-test-extends-imported.yaml")
	assert.Equal(t, filepath.ToSlash(p.Services["service"].EnvFiles[0].Path), "testdata/subdir/extra.env")
}

func TestNestedIncludeAndExtends(t *testing.T) {
	fileName := "compose.yml"
	yaml := `
include:
  - project_directory: .
    path: dir/included.yaml
`
	tmpdir := t.TempDir()
	path := createFile(t, tmpdir, yaml, fileName)

	yaml = `
services:
  included:
    extends:
      file: dir/extended.yaml
      service: extended
`
	createFileSubDir(t, tmpdir, "dir", yaml, "included.yaml")

	yaml = `
services:
  extended:
    image: alpine
`
	createFile(t, filepath.Join(tmpdir, "dir"), yaml, "extended.yaml")
	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: path,
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.SetProjectName("project", true)
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["included"].Image, "alpine")
}

func createFile(t *testing.T, rootDir, content, fileName string) string {
	path := filepath.Join(rootDir, fileName)
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func createFileSubDir(t *testing.T, rootDir, subDir, content, fileName string) {
	subDirPath := filepath.Join(rootDir, subDir)
	assert.NilError(t, os.Mkdir(subDirPath, 0o700))
	path := filepath.Join(subDirPath, fileName)
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
}

// TestIncludeDiamondDedup builds a deep "diamond" include graph where every
// level includes the next level twice. Without include memoization the leaf is
// loaded 2^depth times (exponential); the cache loads each distinct file once.
// A depth that is trivial when deduplicated (and astronomically large when not)
// makes this both a correctness and a non-flaky performance regression test.
func TestIncludeDiamondDedup(t *testing.T) {
	dir := t.TempDir()
	const depth = 24 // 2^24 ~= 16.7M leaf loads without dedup
	for i := 0; i < depth; i++ {
		content := fmt.Sprintf("include:\n  - path: ./level%d.yaml\n  - path: ./level%d.yaml\n", i+1, i+1)
		assert.NilError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("level%d.yaml", i)), []byte(content), 0o600))
	}
	leaf := "services:\n  leaf:\n    image: busybox\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("level%d.yaml", depth)), []byte(leaf), 0o600))

	type result struct {
		p   *types.Project
		err error
	}

	// loader doesn't check ctx.Done() during expansion, so a context timeout
	// wouldn't interrupt a regressed (exponential) load — the goroutine would
	// stay parked and the test would hang until the 10-minute `go test` default,
	// failing with a generic timeout. Running the load off the test goroutine and
	// selecting on a separate timer gives a fast, descriptive failure instead. The
	// leaked goroutine on timeout is harmless: t.Fatal ends the test immediately.
	timeout, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	done := make(chan result, 1)
	go func() {
		p, err := LoadWithContext(t.Context(), types.ConfigDetails{
			WorkingDir:  dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "level0.yaml")}},
		}, withProjectName("diamond", true))
		done <- result{p, err}
	}()

	select {
	case r := <-done:
		assert.NilError(t, r.err)
		_, err := r.p.GetService("leaf")
		assert.NilError(t, err)
	case <-timeout.Done():
		t.Fatal("diamond include did not complete within 30s — include memoization likely regressed")
	}
}

// TestIncludeDiamondListener pins the public Listener contract under include
// memoization: a listener event emitted while expanding an included file must
// fire once per include occurrence, even when the file is served from the
// include cache. shared.yaml is reached through both a.yaml and b.yaml (a
// diamond) and carries one `extends`; a faithful load emits "extends" twice
// (once per path), exactly as loading shared.yaml twice without a cache would.
// Memoizing the load must replay the recorded event on the cache hit rather than
// silently dropping it — otherwise the emitted count would depend on include
// topology. Without the recordings replayed in ApplyInclude this asserts 1.
func TestIncludeDiamondListener(t *testing.T) {
	dir := t.TempDir()
	createFile(t, dir, "include:\n  - path: ./a.yaml\n  - path: ./b.yaml\n", "root.yaml")
	createFile(t, dir, "include:\n  - path: ./shared.yaml\n", "a.yaml")
	createFile(t, dir, "include:\n  - path: ./shared.yaml\n", "b.yaml")
	createFile(t, dir, "services:\n  base:\n    image: alpine\n  derived:\n    extends: base\n    command: echo\n", "shared.yaml")

	var extendsCount, includeCount int
	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "root.yaml")}},
	}, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.SetProjectName("diamond-listener", true)
		options.Listeners = []Listener{
			func(event string, _ map[string]any) {
				switch event {
				case "extends":
					extendsCount++
				case "include":
					includeCount++
				}
			},
		}
	})
	assert.NilError(t, err)
	_, err = p.GetService("derived")
	assert.NilError(t, err)

	// One "extends" per traversal of shared.yaml (via a.yaml and via b.yaml).
	assert.Equal(t, extendsCount, 2)
	// "include" fires per occurrence in the outer loop (root→a, root→b, a→shared,
	// b→shared); it is unaffected by the cache and pins that as the baseline.
	assert.Equal(t, includeCount, 4)
}

// TestIncludeKeyNoCollision pins the length/count-prefixed encoding of the
// include cache key: distinct (paths, workingDir, projectDir, env) tuples must
// never hash to the same key. A bare-separator encoding can collide when a value
// contains the separator byte (env comes from .env files / process env, where
// any byte is legal) or when a field's content spills across a positional
// boundary. A collision would serve a wrong cached model with no error surfaced.
func TestIncludeKeyNoCollision(t *testing.T) {
	base := includeKey([]string{"compose.yaml"}, "/wd", "/pd", types.Mapping{"A": "B"})

	cases := map[string]string{
		// Reviewer's NUL toy example: both serialize identically under a bare
		// NUL separator, but the keys must differ.
		"nul in key vs value a": includeKey([]string{"compose.yaml"}, "/wd", "/pd", types.Mapping{"A\x00B": "X"}),
		"nul in key vs value b": includeKey([]string{"compose.yaml"}, "/wd", "/pd", types.Mapping{"A": "B\x00X"}),
		// Field content that could impersonate an adjacent field/boundary.
		"path absorbs workingdir": includeKey([]string{"compose.yaml", "/wd"}, "/pd", "", types.Mapping{}),
		"env absorbs fields":      includeKey([]string{"compose.yaml"}, "/wd", "/pd", types.Mapping{"X": "Y"}),
		// Plain distinct inputs.
		"different env value":  includeKey([]string{"compose.yaml"}, "/wd", "/pd", types.Mapping{"A": "C"}),
		"different workingdir": includeKey([]string{"compose.yaml"}, "/other", "/pd", types.Mapping{"A": "B"}),
	}

	seen := map[string]string{base: "base"}
	for name, key := range cases {
		if prev, ok := seen[key]; ok {
			t.Fatalf("include key collision between %q and %q", prev, name)
		}
		seen[key] = name
	}

	// Identical inputs must produce identical keys (the cache hit path).
	assert.Equal(t, base, includeKey([]string{"compose.yaml"}, "/wd", "/pd", types.Mapping{"A": "B"}))
}

func BenchmarkIncludeDiamond(b *testing.B) {
	dir := b.TempDir()
	const depth = 16
	for i := 0; i < depth; i++ {
		content := fmt.Sprintf("include:\n  - path: ./level%d.yaml\n  - path: ./level%d.yaml\n", i+1, i+1)
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("level%d.yaml", i)), []byte(content), 0o600)
	}
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("level%d.yaml", depth)), []byte("services:\n  leaf:\n    image: busybox\n"), 0o600)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
			WorkingDir:  dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "level0.yaml")}},
		}, withProjectName("diamond", true))
		if err != nil {
			b.Fatal(err)
		}
	}
}
