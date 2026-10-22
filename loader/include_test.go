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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestLoadIncludeExtendsCombined(t *testing.T) {
	_, err := LoadWithContext(t.Context(), types.ConfigDetails{
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

	p, err := LoadWithContext(t.Context(), details, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	imported, err := p.GetService("imported")
	assert.NilError(t, err)
	assert.Equal(t, imported.ContainerName, "override")
}

func TestLoadWithDiamondInclude(t *testing.T) {
	// the same file reached through multiple include paths must merge
	// idempotently — re-merging identical definitions duplicated entries in
	// list-valued extension fields
	details := buildConfigDetails(`
name: 'test-diamond-include'

include:
  - path: ./testdata/include-diamond-left.yaml
  - path: ./testdata/include-diamond-right.yaml
`, nil)

	p, err := LoadWithContext(t.Context(), details, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	diamond, err := p.GetService("diamond")
	assert.NilError(t, err)
	xbake, ok := diamond.Build.Extensions["x-bake"].(map[string]any)
	assert.Assert(t, ok)
	cacheFrom, ok := xbake["cache-from"].([]any)
	assert.Assert(t, ok)
	assert.Equal(t, len(cacheFrom), 1)
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
	p, err := LoadWithContext(t.Context(), details, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["bar"], types.ServiceConfig{
		Name: "bar",
		ContainerSpec: types.ContainerSpec{
			Image: "busybox",
			Environment: types.MappingWithEquals{
				"ZOT": strPtr("QIX"),
			},
		},
	})
}

func TestIncludeRelative(t *testing.T) {
	wd, err := filepath.Abs(filepath.Join("testdata", "include"))
	assert.NilError(t, err)
	p, err := LoadWithContext(t.Context(), types.ConfigDetails{
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

	p, err := LoadWithContext(t.Context(), types.ConfigDetails{
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
	p, err := LoadWithContext(t.Context(), types.ConfigDetails{
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
	p, err := LoadWithContext(t.Context(), types.ConfigDetails{
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

// includeProject loads the compose file `content` written in a fresh
// directory, next to the extra files given by name, and returns the error
// instead of failing so callers can assert on it.
func includeProject(t *testing.T, content string, files map[string]string, opts ...func(*Options)) (*types.Project, error) {
	t.Helper()
	dir := t.TempDir()
	for name, c := range files {
		assert.NilError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700))
		createFile(t, dir, c, name)
	}
	path := createFile(t, dir, content, "compose.yaml")
	return LoadWithContext(t.Context(), types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: path}},
	}, append([]func(*Options){func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.SetProjectName("project", true)
	}}, opts...)...)
}

// `include` must be a list: anything else is rejected before any file is read.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md
func TestIncludeMustBeAList(t *testing.T) {
	_, err := includeProject(t, "include: other.yaml\n", nil)
	assert.ErrorContains(t, err, "`include` must be a list, got other.yaml")
}

// An included project is interpolated with the `.env` file found in its
// project directory, unless the include declares its own `env_file`. These
// variables stay private to the included project.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeDefaultEnvFile(t *testing.T) {
	p, err := includeProject(t, `
include:
  - path: ./module/compose.yaml
services:
  a:
    image: "${IMAGE:-outer}"
`, map[string]string{
		"module/compose.yaml": "services:\n  b:\n    image: \"${IMAGE:-inner}\"\n",
		"module/.env":         "IMAGE=from-dotenv\n",
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["a"].Image, "outer")
	assert.Equal(t, p.Services["b"].Image, "from-dotenv")
}

// Declaring `env_file` on an include replaces the project directory's `.env`
// default, and `/dev/null` explicitly opts out of any env file.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#env_file
func TestIncludeEnvFileDevNull(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null is not an absolute path on windows")
	}
	p, err := includeProject(t, `
include:
  - path: ./module/compose.yaml
    env_file: /dev/null
`, map[string]string{
		"module/compose.yaml": "services:\n  b:\n    image: \"${IMAGE:-inner}\"\n",
		"module/.env":         "IMAGE=from-dotenv\n",
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["b"].Image, "inner")
}

type failingLoader struct{}

func (failingLoader) Accept(p string) bool { return strings.HasPrefix(p, "fail:") }
func (failingLoader) Load(context.Context, string) (string, error) {
	return "", errors.New("cannot fetch resource")
}
func (failingLoader) Dir(string) string { return "" }

// Every way an include can be unusable is reported to the caller, unchanged:
// an env_file that is missing or is a directory, a failing resource loader, an
// included file that cannot be parsed, and two includes that define the same
// resource with incompatible content.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md
func TestIncludeErrors(t *testing.T) {
	tests := []struct {
		name    string
		compose string
		files   map[string]string
		opts    []func(*Options)
		err     string
	}{
		{
			name:    "relative env_file does not exist",
			compose: "include:\n  - path: ./module/compose.yaml\n    env_file: missing.env\n",
			files:   map[string]string{"module/compose.yaml": "services: {b: {image: x}}\n"},
			err:     "missing.env",
		},
		{
			name:    "absolute env_file does not exist",
			compose: "include:\n  - path: ./module/compose.yaml\n    env_file: '" + filepath.Join(t.TempDir(), "absent.env") + "'\n",
			files:   map[string]string{"module/compose.yaml": "services: {b: {image: x}}\n"},
			err:     "couldn't find env file",
		},
		{
			name:    "relative env_file is a directory",
			compose: "include:\n  - path: ./module/compose.yaml\n    env_file: module\n",
			files:   map[string]string{"module/compose.yaml": "services: {b: {image: x}}\n"},
			err:     "is not a file",
		},
		{
			name:    "resource loader failure",
			compose: "include:\n  - path: fail:module\n",
			opts:    []func(*Options){func(o *Options) { o.ResourceLoaders = []ResourceLoader{failingLoader{}} }},
			err:     "cannot fetch resource",
		},
		{
			name:    "included file cannot be parsed",
			compose: "include:\n  - path: ./module/compose.yaml\n",
			files:   map[string]string{"module/compose.yaml": "services: [\n"},
			err:     "failed to parse",
		},
		{
			name:    "included file is invalid",
			compose: "include:\n  - path: ./module/compose.yaml\n",
			files:   map[string]string{"module/compose.yaml": "services:\n  b:\n    unknown_attribute: true\n"},
			err:     "unknown_attribute",
		},
		{
			name:    "conflicting resources",
			compose: "include:\n  - ./one.yaml\n  - ./two.yaml\n",
			files: map[string]string{
				"one.yaml": "services:\n  b:\n    image: x\n    logging:\n      driver: json-file\n",
				"two.yaml": "services:\n  b:\n    image: x\n    logging: not-a-mapping\n",
			},
			opts: []func(*Options){func(o *Options) { o.SkipValidation = true }},
			err:  "services.b.logging: cannot merge logging",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := includeProject(t, tt.compose, tt.files, tt.opts...)
			assert.ErrorContains(t, err, tt.err)
		})
	}
}

// A resource loader that does not accept an include path leaves it to the
// next loaders: local paths are still resolved by the default local loader.
// https://github.com/compose-spec/compose-spec/blob/main/14-include.md#path
func TestIncludeSkipsLoadersNotAcceptingPath(t *testing.T) {
	p, err := includeProject(t, "include:\n  - ./module/compose.yaml\n",
		map[string]string{"module/compose.yaml": "services:\n  b:\n    image: x\n"},
		func(o *Options) { o.ResourceLoaders = []ResourceLoader{failingLoader{}} })
	assert.NilError(t, err)
	assert.Equal(t, p.Services["b"].Image, "x")
}
