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
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func TestProjectName(t *testing.T) {
	opts, err := NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithName("my_project"))
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	assert.Equal(t, p.Name, "my_project")

	opts, err = NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithWorkingDirectory("."))
	assert.NilError(t, err)
	p, err = ProjectFromOptions(opts)
	assert.NilError(t, err)
	assert.Equal(t, p.Name, "cli")

	opts, err = NewProjectOptions([]string{"testdata/simple/compose.yaml"})
	assert.NilError(t, err)
	p, err = ProjectFromOptions(opts)
	assert.NilError(t, err)
	assert.Equal(t, p.Name, "simple")

	os.Setenv("COMPOSE_PROJECT_NAME", "my_project_from_env")
	defer os.Unsetenv("COMPOSE_PROJECT_NAME")
	opts, err = NewProjectOptions([]string{"testdata/simple/compose.yaml"}, WithOsEnv)
	assert.NilError(t, err)
	p, err = ProjectFromOptions(opts)
	assert.NilError(t, err)
	assert.Equal(t, p.Name, "my_project_from_env")
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

func TestProjectWithDotEnv(t *testing.T) {
	opts, err := NewProjectOptions([]string{
		"testdata/simple/compose-with-variables.yaml",
	}, WithName("my_project"), WithDotEnv)
	assert.NilError(t, err)
	p, err := ProjectFromOptions(opts)
	assert.NilError(t, err)
	service, err := p.GetService("simple")
	assert.NilError(t, err)
	assert.Equal(t, service.Ports[0].Published, uint32(8000))
}

func TestEnvMap(t *testing.T) {
	m := map[string]string{}
	m["foo"] = "bar"
	l := getAsStringList(m)
	assert.Equal(t, l[0], "foo=bar")
	m = getAsEqualsMap(l)
	assert.Equal(t, m["foo"], "bar")
}
