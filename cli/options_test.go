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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"

	"github.com/compose-spec/compose-go/v2/consts"
	"github.com/compose-spec/compose-go/v2/utils"
)

func TestProjectName(t *testing.T) {
	t.Run("by name", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("my_project"))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "my_project")
	})

	t.Run("by name start with number", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("42my_project_num"))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "42my_project_num")

		opts, err = NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "42my_project_env"),
		}))
		assert.NilError(t, err)
		p, err = ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "42my_project_env")
	})

	t.Run("by name empty", func(t *testing.T) {
		opts, err := NewProjectOptions(
			[]string{"testdata/simple/compose.yaml"},
			WithName(""),
		)
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "simple")
	})

	t.Run("by name empty working dir", func(t *testing.T) {
		opts, err := NewProjectOptions(
			[]string{"testdata/simple/compose.yaml"},
			WithName(""),
			WithWorkingDirectory("/path/to/proj"),
		)
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "proj")
	})

	t.Run("by name must not come from root directory", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"},
			WithWorkingDirectory("/"))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)

		// root directory will resolve to an empty project name since there
		// IS no directory name!
		assert.ErrorContains(t, err, `project name must not be empty`)
		assert.Assert(t, p == nil)
	})

	t.Run("by name start with invalid char '-'", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("-my_project"))
		assert.ErrorContains(t, err, `invalid project name "-my_project"`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "-my_project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.ErrorContains(t, err, `invalid project name "-my_project"`)
		assert.Assert(t, p == nil)
	})

	t.Run("by name start with invalid char '_'", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("_my_project"))
		assert.ErrorContains(t, err, `invalid project name "_my_project"`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "_my_project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.ErrorContains(t, err, `invalid project name "_my_project"`)
		assert.Assert(t, p == nil)
	})

	t.Run("by name contains dots", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("www.my.project"))
		assert.ErrorContains(t, err, `invalid project name "www.my.project"`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "www.my.project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.ErrorContains(t, err, `invalid project name "www.my.project"`)
		assert.Assert(t, p == nil)
	})

	t.Run("by name uppercase", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("MY_PROJECT"))
		assert.ErrorContains(t, err, `invalid project name "MY_PROJECT"`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "MY_PROJECT"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.ErrorContains(t, err, `invalid project name "MY_PROJECT"`)
		assert.Assert(t, p == nil)
	})

	t.Run("by working dir", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithWorkingDirectory("."))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "cli")
	})

	t.Run("by compose file parent dir", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"})
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "simple")
	})

	t.Run("by compose file parent dir special", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/UNNORMALIZED PATH/compose.yaml"})
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "unnormalizedpath")
	})

	t.Run("by COMPOSE_PROJECT_NAME", func(t *testing.T) {
		os.Setenv("COMPOSE_PROJECT_NAME", "my_project_from_env") //nolint:errcheck
		defer os.Unsetenv("COMPOSE_PROJECT_NAME")                //nolint:errcheck
		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithOsEnv)
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "my_project_from_env")
	})

	t.Run("by .env", func(t *testing.T) {
		wd, err := os.Getwd()
		assert.NilError(t, err)
		err = os.Chdir("testdata/env-file")
		assert.NilError(t, err)
		defer os.Chdir(wd) //nolint:errcheck

		opts, err := NewProjectOptions(nil, WithDotEnv, WithConfigFileEnv)
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "my_project_from_dot_env")
	})

	t.Run("by name in compose.yaml with variable", func(t *testing.T) {
		opts, err := NewProjectOptions([]string{"testdata/simple/compose-name.yaml"}, WithEnv([]string{
			"TEST=expected",
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "test-expected-test")
	})
}

func TestProjectFromSetOfFiles(t *testing.T) {
	opts, err := NewProjectOptions([]string{
		"testdata/simple/compose.yaml",
		"testdata/simple/compose-with-overrides.yaml",
	}, WithName("my_project"))
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	service, err := p.GetService("simple")
	assert.NilError(t, err)
	assert.Equal(t, service.Image, "haproxy")
}

func TestProjectComposefilesFromSetOfFiles(t *testing.T) {
	opts, err := NewProjectOptions([]string{},
		WithWorkingDirectory("testdata/simple/"),
		WithName("my_project"),
		WithDefaultConfigPath,
	)
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	absPath, _ := filepath.Abs(filepath.Join("testdata", "simple", "compose.yaml"))
	assert.DeepEqual(t, p.ComposeFiles, []string{absPath})
}

func TestProjectComposefilesFromWorkingDir(t *testing.T) {
	opts, err := NewProjectOptions([]string{
		"testdata/simple/compose.yaml",
		"testdata/simple/compose-with-overrides.yaml",
	}, WithName("my_project"))
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	currentDir, _ := os.Getwd()
	assert.DeepEqual(t, p.ComposeFiles, []string{
		filepath.Join(currentDir, "testdata", "simple", "compose.yaml"),
		filepath.Join(currentDir, "testdata", "simple", "compose-with-overrides.yaml"),
	})
}

func TestProjectWithDotEnv(t *testing.T) {
	wd, err := os.Getwd()
	assert.NilError(t, err)
	err = os.Chdir("testdata/simple")
	assert.NilError(t, err)
	defer os.Chdir(wd) //nolint:errcheck

	opts, err := NewProjectOptions([]string{
		"compose-with-variables.yaml",
	}, WithName("my_project"), WithDotEnv)
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	service, err := p.GetService("simple")
	assert.NilError(t, err)
	assert.Equal(t, service.Ports[0].Published, "8000")
}

func TestProjectWithDiscardEnvFile(t *testing.T) {
	opts, err := NewProjectOptions([]string{
		"testdata/env-file/compose-with-env-file.yaml",
	}, WithDiscardEnvFile)

	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	service, err := p.GetService("simple")
	assert.NilError(t, err)
	assert.Equal(t, *service.Environment["DEFAULT_PORT"], "8080")
	assert.Assert(t, service.EnvFile == nil)
	assert.Equal(t, service.Ports[0].Published, "8000")
}

func TestProjectWithMultipleEnvFile(t *testing.T) {
	opts, err := NewProjectOptions([]string{
		"testdata/env-file/compose-with-env-files.yaml",
	}, WithDiscardEnvFile,
		WithEnvFiles("testdata/env-file/.env", "testdata/env-file/override.env"),
		WithDotEnv)

	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	service, err := p.GetService("simple")
	assert.NilError(t, err)
	assert.Equal(t, *service.Environment["DEFAULT_PORT"], "9090")
	assert.Assert(t, service.EnvFile == nil)
	assert.Equal(t, service.Ports[0].Published, "9000")
}

func TestProjectNameFromWorkingDir(t *testing.T) {
	opts, err := NewProjectOptions([]string{
		"testdata/env-file/compose-with-env-file.yaml",
	})
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	assert.Equal(t, p.Name, "env-file")
}

func TestEnvMap(t *testing.T) {
	m := map[string]string{}
	m["foo"] = "bar"
	l := utils.GetAsStringList(m)
	assert.Equal(t, l[0], "foo=bar")
	m = utils.GetAsEqualsMap(l)
	assert.Equal(t, m["foo"], "bar")
}

func TestEnvVariablePrecedence(t *testing.T) {
	testcases := []struct {
		name     string
		dotEnv   string
		osEnv    []string
		expected types.Mapping
	}{
		{
			"no value set in environment",
			"FOO=foo\nBAR=${FOO}",
			nil,
			types.Mapping{
				"FOO": "foo",
				"BAR": "foo",
			},
		},
		{
			"conflict with value set in environment",
			"FOO=foo\nBAR=${FOO}",
			[]string{"FOO=zot"},
			types.Mapping{
				"FOO": "zot",
				"BAR": "zot",
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			wd := t.TempDir()
			err := os.WriteFile(filepath.Join(wd, ".env"), []byte(test.dotEnv), 0o700)
			assert.NilError(t, err)
			options, err := NewProjectOptions(nil,
				// First load os.Env variable, higher in precedence rule
				WithEnv(test.osEnv),
				// Then load dotEnv file
				WithWorkingDirectory(wd), WithDotEnv)
			assert.NilError(t, err)
			assert.DeepEqual(t, test.expected, options.Environment)
		})
	}
}
