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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func lazyLoad(t *testing.T, yaml string, env map[string]string, options ...func(*Options)) *types.Project {
	t.Helper()
	if env == nil {
		env = map[string]string{}
	}
	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: "compose.yml", Content: []byte(yaml)},
		},
		Environment: env,
	}
	model, err := LoadLazyModel(t.Context(), configDetails, options...)
	assert.NilError(t, err)

	project, err := model.Resolve(t.Context())
	assert.NilError(t, err)
	return project
}

func TestLoadLazyModel_Simple(t *testing.T) {
	yaml := `
name: testproject
services:
  web:
    image: nginx
    ports:
      - "8080:80"
  db:
    image: postgres
    environment:
      POSTGRES_DB: mydb
`
	project := lazyLoad(t, yaml, nil, func(o *Options) {
		o.SkipConsistencyCheck = true
	})

	assert.Equal(t, project.Name, "testproject")
	assert.Equal(t, len(project.Services), 2)

	web := project.Services["web"]
	assert.Equal(t, web.Image, "nginx")

	db := project.Services["db"]
	assert.Equal(t, db.Image, "postgres")

	val, ok := db.Environment["POSTGRES_DB"]
	assert.Assert(t, ok)
	assert.Assert(t, val != nil)
	assert.Equal(t, *val, "mydb")
}

func TestLoadLazyModel_Interpolation(t *testing.T) {
	yaml := `
name: interpoltest
services:
  app:
    image: ${APP_IMAGE}
`
	project := lazyLoad(t, yaml, map[string]string{
		"APP_IMAGE": "myapp:latest",
	}, func(o *Options) {
		o.SkipConsistencyCheck = true
	})

	app := project.Services["app"]
	assert.Equal(t, app.Image, "myapp:latest")
}

func TestLoadLazyModel_BuildShortSyntax(t *testing.T) {
	yaml := `
name: buildtest
services:
  app:
    build: ./app
`
	project := lazyLoad(t, yaml, nil, func(o *Options) {
		o.SkipConsistencyCheck = true
	})

	app := project.Services["app"]
	assert.Assert(t, app.Build != nil)
	assert.Assert(t, strings.Contains(app.Build.Context, "app"),
		"expected Build.Context to contain 'app', got: %s", app.Build.Context)
}

func TestLoadLazyModel_DependsOnShortSyntax(t *testing.T) {
	yaml := `
name: depstest
services:
  web:
    image: nginx
    depends_on:
      - db
  db:
    image: postgres
`
	project := lazyLoad(t, yaml, nil, func(o *Options) {
		o.SkipConsistencyCheck = true
	})

	web := project.Services["web"]
	dep, ok := web.DependsOn["db"]
	assert.Assert(t, ok, "expected depends_on to contain 'db'")
	assert.Equal(t, dep.Condition, "service_started")
	assert.Equal(t, dep.Required, true)
}

func TestLoadLazyModel_Parity(t *testing.T) {
	yaml := `
name: paritytest
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    environment:
      - FOO=bar
    labels:
      app: web
    depends_on:
      - db
  db:
    image: postgres:15
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: user
volumes:
  data: {}
networks:
  frontend: {}
`
	sharedOpts := func(o *Options) {
		o.SkipConsistencyCheck = true
		o.SkipNormalization = true
		o.ResolvePaths = true
	}

	env := map[string]string{}
	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: "compose.yml", Content: []byte(yaml)},
		},
		Environment: env,
	}

	// Load with existing pipeline
	projectOld, err := LoadWithContext(t.Context(), configDetails, sharedOpts)
	assert.NilError(t, err)

	// Load with new lazy pipeline
	model, err := LoadLazyModel(t.Context(), configDetails, sharedOpts)
	assert.NilError(t, err)
	projectNew, err := model.Resolve(t.Context())
	assert.NilError(t, err)

	// Compare project names
	assert.Equal(t, projectOld.Name, projectNew.Name)

	// Compare service names
	oldNames := projectOld.ServiceNames()
	newNames := projectNew.ServiceNames()
	sort.Strings(oldNames)
	sort.Strings(newNames)
	assert.DeepEqual(t, oldNames, newNames)

	// Compare images
	for _, name := range oldNames {
		oldSvc := projectOld.Services[name]
		newSvc := projectNew.Services[name]
		assert.Equal(t, oldSvc.Image, newSvc.Image, "image mismatch for service %s", name)
	}

	// Compare environment values
	for _, name := range oldNames {
		oldSvc := projectOld.Services[name]
		newSvc := projectNew.Services[name]
		for key, oldVal := range oldSvc.Environment {
			newVal, ok := newSvc.Environment[key]
			assert.Assert(t, ok, "env key %s missing in service %s", key, name)
			if oldVal != nil && newVal != nil {
				assert.Equal(t, *oldVal, *newVal, "env %s mismatch for service %s", key, name)
			}
		}
	}

	// Compare labels
	oldWeb := projectOld.Services["web"]
	newWeb := projectNew.Services["web"]
	assert.DeepEqual(t, map[string]string(oldWeb.Labels), map[string]string(newWeb.Labels))

	// Compare depends_on conditions
	for depName, oldDep := range oldWeb.DependsOn {
		newDep, ok := newWeb.DependsOn[depName]
		assert.Assert(t, ok, "depends_on %s missing", depName)
		assert.Equal(t, oldDep.Condition, newDep.Condition)
		assert.Equal(t, oldDep.Required, newDep.Required)
	}

	// Compare volumes and networks exist
	for name := range projectOld.Volumes {
		_, ok := projectNew.Volumes[name]
		assert.Assert(t, ok, "volume %s missing in new project", name)
	}
	for name := range projectOld.Networks {
		_, ok := projectNew.Networks[name]
		assert.Assert(t, ok, "network %s missing in new project", name)
	}
}

func TestLoadLazyModel_ErrorIncludesLineInfo(t *testing.T) {
	// This YAML has an invalid value for "ports" — a boolean instead of string/number.
	// The error should include line and column information.
	yamlContent := `name: errtest
services:
  web:
    image: nginx
    mem_limit: not_a_size
`
	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: "docker-compose.yml", Content: []byte(yamlContent)},
		},
		Environment: map[string]string{},
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	_, err = model.Resolve(t.Context())
	assert.Assert(t, err != nil, "expected an error for invalid mem_limit")

	errMsg := err.Error()
	t.Logf("error message: %s", errMsg)

	// The error message should contain the source filename and line/column info
	assert.Assert(t, strings.Contains(errMsg, "docker-compose.yml"),
		"expected error to contain source filename")
	assert.Assert(t, strings.Contains(errMsg, "line 5"),
		"expected error to contain line number")
	assert.Assert(t, strings.Contains(errMsg, "column 16"),
		"expected error to contain column number")
}

func TestLoadLazyModel_ErrorIncludesLineInfo_InvalidDuration(t *testing.T) {
	yamlContent := `name: errtest
services:
  web:
    image: nginx
    healthcheck:
      test: ["CMD", "true"]
      interval: not_a_duration
`
	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: "myapp/compose.yaml", Content: []byte(yamlContent)},
		},
		Environment: map[string]string{},
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	_, err = model.Resolve(t.Context())
	assert.Assert(t, err != nil, "expected an error for invalid duration")

	errMsg := err.Error()
	t.Logf("error message: %s", errMsg)

	// The error message should contain the source filename and line/column info
	assert.Assert(t, strings.Contains(errMsg, "myapp/compose.yaml"),
		"expected error to contain source filename")
	assert.Assert(t, strings.Contains(errMsg, "line 7"),
		"expected error to contain line number")
	assert.Assert(t, strings.Contains(errMsg, "column 17"),
		"expected error to contain column number")
}

func TestLoadLazyModel_ErrorMultipleFiles(t *testing.T) {
	// First file is valid, second file has an error on a specific line.
	// The error should reference the second file's name and the correct line.
	base := `name: multitest
services:
  web:
    image: nginx
`
	overrideContent := `services:
  web:
    mem_limit: not_a_size
`
	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: "docker-compose.yml", Content: []byte(base)},
			{Filename: "docker-compose.override.yml", Content: []byte(overrideContent)},
		},
		Environment: map[string]string{},
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	_, err = model.Resolve(t.Context())
	assert.Assert(t, err != nil, "expected an error for invalid mem_limit in override")

	errMsg := err.Error()
	t.Logf("error message: %s", errMsg)

	// The error should reference the override file (last file merged)
	assert.Assert(t, strings.Contains(errMsg, "docker-compose.override.yml"),
		"expected error to contain override filename, got: %s", errMsg)
	// Should contain line info
	assert.Assert(t, strings.Contains(errMsg, "line "),
		"expected error to contain line number, got: %s", errMsg)
	assert.Assert(t, strings.Contains(errMsg, "column "),
		"expected error to contain column number, got: %s", errMsg)
}

func TestLoadLazyModel_ErrorExtends(t *testing.T) {
	// Create a temporary directory with a base file containing an error.
	// The main file extends from it. The error should reference the base file.
	tmpDir := t.TempDir()

	baseContent := `services:
  base:
    image: nginx
    mem_limit: not_a_size
`
	err := os.WriteFile(filepath.Join(tmpDir, "base.yml"), []byte(baseContent), 0o644)
	assert.NilError(t, err)

	mainContent := `name: extendstest
services:
  web:
    extends:
      file: base.yml
      service: base
`
	configDetails := types.ConfigDetails{
		WorkingDir: tmpDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: filepath.Join(tmpDir, "compose.yml"), Content: []byte(mainContent)},
		},
		Environment: map[string]string{},
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	_, err = model.Resolve(t.Context())
	assert.Assert(t, err != nil, "expected an error for invalid mem_limit from extended service")

	errMsg := err.Error()
	t.Logf("error message: %s", errMsg)

	// The error should contain line info (from the yaml node that has the bad value)
	assert.Assert(t, strings.Contains(errMsg, "line "),
		"expected error to contain line number, got: %s", errMsg)
	assert.Assert(t, strings.Contains(errMsg, "column "),
		"expected error to contain column number, got: %s", errMsg)
}

func TestLoadLazyModel_IncludeEnvFile(t *testing.T) {
	// An included file uses a variable ${WORKER_IMAGE} that is defined only
	// in the env_file specified by the include directive. The main file does
	// NOT have this variable in its environment. The included layer must be
	// interpolated with the env_file values.
	tmpDir := t.TempDir()

	// Create the env file with a variable unknown to the main file
	err := os.WriteFile(filepath.Join(tmpDir, "worker.env"), []byte("WORKER_IMAGE=redis:7\n"), 0o644)
	assert.NilError(t, err)

	// Included compose file uses ${WORKER_IMAGE}
	includedContent := `services:
  worker:
    image: ${WORKER_IMAGE}
`
	err = os.WriteFile(filepath.Join(tmpDir, "worker.yml"), []byte(includedContent), 0o644)
	assert.NilError(t, err)

	// Main compose file includes worker.yml with env_file
	mainContent := `name: incenvtest
include:
  - path: worker.yml
    env_file: worker.env
services:
  web:
    image: nginx
`
	configDetails := types.ConfigDetails{
		WorkingDir: tmpDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: filepath.Join(tmpDir, "compose.yml"), Content: []byte(mainContent)},
		},
		Environment: map[string]string{},
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	project, err := model.Resolve(t.Context())
	assert.NilError(t, err)

	// The main file's web service should be present
	web := project.Services["web"]
	assert.Equal(t, web.Image, "nginx")

	// The included worker service should have been interpolated with WORKER_IMAGE from worker.env
	worker, ok := project.Services["worker"]
	assert.Assert(t, ok, "expected 'worker' service from included file")
	assert.Equal(t, worker.Image, "redis:7",
		"expected included service to be interpolated with env_file variable, got: %s", worker.Image)
}

func TestLoadLazyModel_ErrorInclude(t *testing.T) {
	// Create a temporary directory with an included file containing an error.
	// The error should reference the included file's name.
	tmpDir := t.TempDir()

	includedContent := `name: included
services:
  worker:
    image: redis
    mem_limit: not_a_size
`
	err := os.WriteFile(filepath.Join(tmpDir, "worker.yml"), []byte(includedContent), 0o644)
	assert.NilError(t, err)

	mainContent := `name: includetest
include:
  - worker.yml
services:
  web:
    image: nginx
`
	configDetails := types.ConfigDetails{
		WorkingDir: tmpDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: filepath.Join(tmpDir, "compose.yml"), Content: []byte(mainContent)},
		},
		Environment: map[string]string{},
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	_, err = model.Resolve(t.Context())
	assert.Assert(t, err != nil, "expected an error for invalid mem_limit in included file")

	errMsg := err.Error()
	t.Logf("error message: %s", errMsg)

	// The error should reference the included file
	assert.Assert(t, strings.Contains(errMsg, "worker.yml"),
		"expected error to contain included filename, got: %s", errMsg)
	// Should contain line info
	assert.Assert(t, strings.Contains(errMsg, "line "),
		"expected error to contain line number, got: %s", errMsg)
	assert.Assert(t, strings.Contains(errMsg, "column "),
		"expected error to contain column number, got: %s", errMsg)
}

func TestLoadLazyModel_MergeEnvironmentMappingAndSequence(t *testing.T) {
	// First file declares environment as a mapping with variable references.
	// Second file declares environment as a sequence with variable references.
	// After merge (convertNodeToSequence creates new scalars from the mapping),
	// all values must be correctly interpolated.
	base := `name: mergeenvtest
services:
  app:
    image: nginx
    environment:
      FROM_MAP: ${MAP_VAR}
      PLAIN_MAP: hello
`
	overrideContent := `services:
  app:
    environment:
      - FROM_SEQ=${SEQ_VAR}
      - PLAIN_SEQ=world
`
	env := map[string]string{
		"MAP_VAR": "map_value",
		"SEQ_VAR": "seq_value",
	}
	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{Filename: "compose.yml", Content: []byte(base)},
			{Filename: "compose.override.yml", Content: []byte(overrideContent)},
		},
		Environment: env,
	}
	model, err := LoadLazyModel(t.Context(), configDetails, func(o *Options) {
		o.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	project, err := model.Resolve(t.Context())
	assert.NilError(t, err)

	app := project.Services["app"]

	// FROM_MAP came from a mapping, was converted to "FROM_MAP=${MAP_VAR}" by merge,
	// then interpolated → "map_value"
	val, ok := app.Environment["FROM_MAP"]
	assert.Assert(t, ok, "FROM_MAP missing")
	assert.Assert(t, val != nil, "FROM_MAP is nil")
	assert.Equal(t, *val, "map_value", "FROM_MAP not interpolated")

	// FROM_SEQ came from a sequence, kept as "FROM_SEQ=${SEQ_VAR}",
	// then interpolated → "seq_value"
	val, ok = app.Environment["FROM_SEQ"]
	assert.Assert(t, ok, "FROM_SEQ missing")
	assert.Assert(t, val != nil, "FROM_SEQ is nil")
	assert.Equal(t, *val, "seq_value", "FROM_SEQ not interpolated")

	// Plain values should pass through unchanged
	val, ok = app.Environment["PLAIN_MAP"]
	assert.Assert(t, ok, "PLAIN_MAP missing")
	assert.Assert(t, val != nil)
	assert.Equal(t, *val, "hello")

	val, ok = app.Environment["PLAIN_SEQ"]
	assert.Assert(t, ok, "PLAIN_SEQ missing")
	assert.Assert(t, val != nil)
	assert.Equal(t, *val, "world")
}
