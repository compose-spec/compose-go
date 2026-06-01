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

package types

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestStringList_UnmarshalYAML_Scalar(t *testing.T) {
	var list StringList
	assert.NilError(t, yaml.Unmarshal([]byte("nginx"), &list))
	assert.DeepEqual(t, list, StringList{"nginx"})
}

func TestStringList_UnmarshalYAML_Sequence(t *testing.T) {
	var list StringList
	assert.NilError(t, yaml.Unmarshal([]byte("- a\n- b\n"), &list))
	assert.DeepEqual(t, list, StringList{"a", "b"})
}

func TestStringList_UnmarshalYAML_InsideStruct(t *testing.T) {
	// Confirm yaml.v4 picks up our UnmarshalYAML when decoding a struct
	// that has a StringList field.
	type wrapper struct {
		Names StringList `yaml:"names"`
	}
	var w wrapper
	assert.NilError(t, yaml.Unmarshal([]byte("names: single"), &w))
	assert.DeepEqual(t, w.Names, StringList{"single"})
	assert.NilError(t, yaml.Unmarshal([]byte("names: [one, two]"), &w))
	assert.DeepEqual(t, w.Names, StringList{"one", "two"})
}

func TestStringOrNumberList_UnmarshalYAML(t *testing.T) {
	var list StringOrNumberList
	assert.NilError(t, yaml.Unmarshal([]byte("- 80\n- \"443\"\n- ssh\n"), &list))
	assert.DeepEqual(t, list, StringOrNumberList{"80", "443", "ssh"})
}

func TestShellCommand_UnmarshalYAML_Scalar(t *testing.T) {
	var cmd ShellCommand
	assert.NilError(t, yaml.Unmarshal([]byte("nginx -g \"daemon off;\""), &cmd))
	assert.DeepEqual(t, cmd, ShellCommand{"nginx", "-g", "daemon off;"})
}

func TestShellCommand_UnmarshalYAML_Sequence(t *testing.T) {
	var cmd ShellCommand
	assert.NilError(t, yaml.Unmarshal([]byte("- nginx\n- -g\n- daemon off;\n"), &cmd))
	assert.DeepEqual(t, cmd, ShellCommand{"nginx", "-g", "daemon off;"})
}

func TestHealthCheckTest_UnmarshalYAML_Scalar(t *testing.T) {
	var test HealthCheckTest
	assert.NilError(t, yaml.Unmarshal([]byte("curl -f http://localhost/"), &test))
	// Short form is wrapped in CMD-SHELL.
	assert.DeepEqual(t, test, HealthCheckTest{"CMD-SHELL", "curl -f http://localhost/"})
}

func TestHealthCheckTest_UnmarshalYAML_Sequence(t *testing.T) {
	var test HealthCheckTest
	assert.NilError(t, yaml.Unmarshal([]byte("- CMD\n- curl\n- -f\n- http://localhost/\n"), &test))
	assert.DeepEqual(t, test, HealthCheckTest{"CMD", "curl", "-f", "http://localhost/"})
}

// TestIncludeConfig_UnmarshalYAML_StringListShortForm confirms that the
// improved StringList unmarshaller lets yaml.v4 decode an include entry
// natively, including the path / env_file scalar short form.
func TestIncludeConfig_UnmarshalYAML_StringListShortForm(t *testing.T) {
	var cfg IncludeConfig
	src := `
path: compose.yaml
project_directory: ./sub
env_file: .env.shared
`
	assert.NilError(t, yaml.Unmarshal([]byte(src), &cfg))
	assert.DeepEqual(t, cfg.Path, StringList{"compose.yaml"})
	assert.Equal(t, cfg.ProjectDirectory, "./sub")
	assert.DeepEqual(t, cfg.EnvFile, StringList{".env.shared"})
}

func TestIncludeConfig_UnmarshalYAML_StringListLongForm(t *testing.T) {
	var cfg IncludeConfig
	src := `
path:
  - first.yaml
  - second.yaml
env_file:
  - .env.a
  - .env.b
`
	assert.NilError(t, yaml.Unmarshal([]byte(src), &cfg))
	assert.DeepEqual(t, cfg.Path, StringList{"first.yaml", "second.yaml"})
	assert.DeepEqual(t, cfg.EnvFile, StringList{".env.a", ".env.b"})
}
