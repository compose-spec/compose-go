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
	"encoding/json"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// TestEnvFile_Marshal_ExcludesContext asserts that EnvFile.Context, the
// loader-populated field used by WithServicesEnvironmentResolved, is not
// serialized to YAML or to JSON. The field is implementation detail and
// must not leak into the project's external representation.
func TestEnvFile_Marshal_ExcludesContext(t *testing.T) {
	ef := EnvFile{
		Path:     "./vars.env",
		Required: OptOut(false),
		Context: &NodeContext{
			Source:     "/abs/path/compose.yaml",
			WorkingDir: "/abs/path",
			Env:        Mapping{"SECRET": "should-not-leak"},
		},
	}

	y, err := yaml.Marshal(ef)
	assert.NilError(t, err)
	yStr := string(y)
	assert.Assert(t, !strings.Contains(yStr, "Context"), "yaml output must not contain field name: %s", yStr)
	assert.Assert(t, !strings.Contains(yStr, "WorkingDir"), "yaml output must not contain WorkingDir: %s", yStr)
	assert.Assert(t, !strings.Contains(yStr, "should-not-leak"), "yaml output must not leak context env: %s", yStr)
	assert.Assert(t, !strings.Contains(yStr, "Source"), "yaml output must not contain Source: %s", yStr)

	j, err := json.Marshal(ef)
	assert.NilError(t, err)
	jStr := string(j)
	assert.Assert(t, !strings.Contains(jStr, "Context"), "json output must not contain field name: %s", jStr)
	assert.Assert(t, !strings.Contains(jStr, "should-not-leak"), "json output must not leak context env: %s", jStr)
}
