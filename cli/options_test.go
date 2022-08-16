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

	"github.com/compose-spec/compose-go/consts"
	"github.com/compose-spec/compose-go/utils"
	"gotest.tools/v3/assert"
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

	t.Run("by name start with invalid char '-'", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("-my_project"))
		assert.Error(t, err, `"-my_project" is not a valid project name`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "-my_project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "my_project")
	})

	t.Run("by name start with invalid char '_'", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("_my_project"))
		assert.Error(t, err, `"_my_project" is not a valid project name`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "_my_project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "my_project")
	})

	t.Run("by name contains dots", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("www.my.project"))
		assert.Error(t, err, `"www.my.project" is not a valid project name`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "www.my.project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "wwwmyproject")
	})

	t.Run("by name uppercase", func(t *testing.T) {
		_, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("MY_PROJECT"))
		assert.Error(t, err, `"MY_PROJECT" is not a valid project name`)

		opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithEnv([]string{
			fmt.Sprintf("%s=%s", consts.ComposeProjectName, "_my_project"),
		}))
		assert.NilError(t, err)
		p, err := ProjectFromOptions(opts)
		assert.NilError(t, err)
		assert.Equal(t, p.Name, "my_project")
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
    t.Run("by UID and GID", func(t *testing.T) {
        opts, err := NewProjectOptions([]string{"testdata/simple/compose-with-uid-gid.yaml"}, WithShellVariables)
        assert.NilError(t, err)
        p, err := ProjectFromOptions(opts)
        service, err := p.GetService("simple")
        assert.NilError(t, err)
        assert.Equal(t, *service.Environment["UID"], os.Getuid())
        assert.Equal(t, *service.Environment["GID"], os.Getgid())
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
