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

package schema

import (
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

func TestValidate(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
			},
		},
	}

	assert.NilError(t, Validate(config))
}

func TestValidateUndefinedTopLevelOption(t *testing.T) {
	config := map[string]any{
		"helicopters": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
			},
		},
	}

	err := Validate(config)
	assert.ErrorContains(t, err, "additional properties 'helicopters' not allowed")
}

func TestValidateAllowsXTopLevelFields(t *testing.T) {
	config := map[string]any{
		"x-extra-stuff": map[string]any{},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateAllowsXFields(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"bar": map[string]any{
				"x-extra-stuff": map[string]any{},
			},
			"foo": map[string]any{
				"depends_on": map[string]any{
					"x-dependency": map[string]any{
						"condition": "service_started",
					},
				},
			},
		},
		"volumes": map[string]any{
			"bar": map[string]any{
				"x-extra-stuff": map[string]any{},
			},
		},
		"networks": map[string]any{
			"bar": map[string]any{
				"x-extra-stuff": map[string]any{},
			},
		},
		"configs": map[string]any{
			"bar": map[string]any{
				"x-extra-stuff": map[string]any{},
			},
		},
		"secrets": map[string]any{
			"bar": map[string]any{
				"x-extra-stuff": map[string]any{},
			},
		},
	}
	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateSecretConfigNames(t *testing.T) {
	config := map[string]any{
		"configs": map[string]any{
			"bar": map[string]any{
				"name": "foobar",
			},
		},
		"secrets": map[string]any{
			"baz": map[string]any{
				"name": "foobaz",
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidatePlacement(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
				"deploy": map[string]any{
					"placement": map[string]any{
						"preferences": []any{
							map[string]any{
								"spread": "node.labels.az",
							},
						},
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateIsolation(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image":     "busybox",
				"isolation": "some-isolation-value",
			},
		},
	}
	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfig(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
				"deploy": map[string]any{
					"rollback_config": map[string]any{
						"parallelism": 1,
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfigWithOrder(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
				"deploy": map[string]any{
					"rollback_config": map[string]any{
						"parallelism": 1,
						"order":       "start-first",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfigWithUpdateConfig(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
				"deploy": map[string]any{
					"update_config": map[string]any{
						"parallelism": 1,
						"order":       "start-first",
					},
					"rollback_config": map[string]any{
						"parallelism": 1,
						"order":       "start-first",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateRollbackConfigWithUpdateConfigFull(t *testing.T) {
	config := map[string]any{
		"services": map[string]any{
			"foo": map[string]any{
				"image": "busybox",
				"deploy": map[string]any{
					"update_config": map[string]any{
						"parallelism":    1,
						"order":          "start-first",
						"delay":          "10s",
						"failure_action": "pause",
						"monitor":        "10s",
					},
					"rollback_config": map[string]any{
						"parallelism":    1,
						"order":          "start-first",
						"delay":          "10s",
						"failure_action": "pause",
						"monitor":        "10s",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config))
	assert.NilError(t, Validate(config))
}

func TestValidateVariables(t *testing.T) {
	bytes, err := os.ReadFile("using-variables.yaml")
	assert.NilError(t, err)
	var config map[string]any
	err = yaml.Unmarshal(bytes, &config)
	assert.NilError(t, err)
	assert.NilError(t, Validate(config))
}

func TestSchema(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	json, err := jsonschema.UnmarshalJSON(strings.NewReader(Schema))
	assert.NilError(t, err)
	err = compiler.AddResource("compose-spec.json", json)
	assert.NilError(t, err)
	compiler.DefaultDraft(jsonschema.Draft7)
	_, err = compiler.Compile("compose-spec.json")
	assert.NilError(t, err)
}
