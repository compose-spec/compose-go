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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
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

// TestInclude_EnvFile_ProvidesContextToServiceEnvFile asserts that each
// env_file entry is interpolated with the environment of the file that
// declared it:
//
//   - extra.env is declared inside the included sub/compose.yaml; its content
//     `FOO=$BAR` resolves against include.env_file (BAR=bar), yielding FOO=bar.
//   - override.env is declared in the top-level compose.yaml as an override of
//     the included `app` service; its content `OVR=${BAR:-fallback}` is
//     interpolated in the top-level scope, where BAR is not defined, so the
//     default value is selected (OVR=fallback).
func TestInclude_EnvFile_ProvidesContextToServiceEnvFile(t *testing.T) {
	workdir, err := filepath.Abs("testdata/include/env_file")
	assert.NilError(t, err)
	topPath := filepath.Join(workdir, "compose.yaml")

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  workdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: map[string]string{},
	}, withProjectName("test-include-envfile-context", true))
	assert.NilError(t, err)

	resolved, err := p.WithServicesEnvironmentResolved(false)
	assert.NilError(t, err)

	app := resolved.Services["app"]

	foo, ok := app.Environment["FOO"]
	assert.Check(t, ok, "FOO should be present in resolved environment")
	if ok && foo != nil {
		assert.Check(t, *foo == "bar", "FOO should be 'bar' (from include.env_file BAR), got %q", *foo)
	}

	ovr, ok := app.Environment["OVR"]
	assert.Check(t, ok, "OVR should be present in resolved environment")
	if ok && ovr != nil {
		assert.Check(t, *ovr == "fallback", "OVR should be 'fallback' (BAR is not visible in top-level scope), got %q", *ovr)
	}
}

// TestInclude_SecretEnvironment_ProvidesContextToSecret asserts that a
// secret declared inside an included file resolves its `environment:`
// variable against the env_file declared on the include block, not the
// parent project environment. Fix for the v2 limitation where
// resolveSecretsEnvironment only looked at the project-wide environment
// and therefore could not see a variable that an include env_file
// introduced inside the include scope.
func TestInclude_SecretEnvironment_ProvidesContextToSecret(t *testing.T) {
	workdir, err := filepath.Abs("testdata/include/secret_env")
	assert.NilError(t, err)
	topPath := filepath.Join(workdir, "compose.yaml")

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  workdir,
		ConfigFiles: []types.ConfigFile{{Filename: topPath}},
		Environment: map[string]string{},
	}, withProjectName("test-include-secret-env", true))
	assert.NilError(t, err)

	secret, ok := p.Secrets["scoped"]
	assert.Assert(t, ok, "secret 'scoped' should be present")
	assert.Equal(t, secret.Environment, "MY_SECRET",
		"secret keeps the environment variable name it was declared with")
	assert.Equal(t, secret.Content, "shadoks",
		"secret content resolves against include env_file MY_SECRET, not parent env")
}
