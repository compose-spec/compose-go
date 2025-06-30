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

package transform

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"go.yaml.in/yaml/v3"
	"gotest.tools/v3/assert"
)

func TestSingle(t *testing.T) {
	env, err := transformEnvFile(".env", tree.NewPath("service.test.env_file"), false)
	assert.NilError(t, err)
	assert.DeepEqual(t, env, []any{
		map[string]any{
			"path":     ".env",
			"required": true,
		},
	})
}

func TestSequence(t *testing.T) {
	var in any
	err := yaml.Unmarshal([]byte(`
  - .env
  - other.env
`), &in)
	assert.NilError(t, err)
	env, err := transformEnvFile(in, tree.NewPath("service.test.env_file"), false)
	assert.NilError(t, err)
	assert.DeepEqual(t, env, []any{
		map[string]any{
			"path":     ".env",
			"required": true,
		},
		map[string]any{
			"path":     "other.env",
			"required": true,
		},
	})
}

func TestOptional(t *testing.T) {
	var in any
	err := yaml.Unmarshal([]byte(`
  - .env
  - path: other.env
    required: false
`), &in)
	assert.NilError(t, err)
	env, err := transformEnvFile(in, tree.NewPath("service.test.env_file"), false)
	assert.NilError(t, err)
	assert.DeepEqual(t, env, []any{
		map[string]any{
			"path":     ".env",
			"required": true,
		},
		map[string]any{
			"path":     "other.env",
			"required": false,
		},
	})
}
