package schema

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

type dict map[string]interface{}

func TestValidate(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	assert.NilError(t, Validate(config, "3"))
}

func TestValidateUndefinedTopLevelOption(t *testing.T) {
	config := dict{
		"version": "3",
		"helicopters": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	err := Validate(config, "3.9")
	assert.ErrorContains(t, err, "Additional property helicopters is not allowed")
}

func TestValidateAllowsXTopLevelFields(t *testing.T) {
	config := dict{
		"version":       "3",
		"x-extra-stuff": dict{},
	}

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateAllowsXFields(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"volumes": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"networks": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"configs": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
		"secrets": dict{
			"bar": dict{
				"x-extra-stuff": dict{},
			},
		},
	}
	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateCredentialSpecs(t *testing.T) {
	tests := []struct {
		version     string
		expectedErr string
	}{
		{version: "3"},
		{version: "3.9"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.version, func(t *testing.T) {
			config := dict{
				"version": "99.99",
				"services": dict{
					"foo": dict{
						"image": "busybox",
						"credential_spec": dict{
							"config": "foobar",
						},
					},
				},
			}
			err := Validate(config, tc.version)
			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, fmt.Sprintf("Additional property %s is not allowed", tc.expectedErr))
			} else {
				assert.NilError(t, err)
			}
		})
	}
}

func TestValidateSecretConfigNames(t *testing.T) {
	config := dict{
		"version": "3",
		"configs": dict{
			"bar": dict{
				"name": "foobar",
			},
		},
		"secrets": dict{
			"baz": dict{
				"name": "foobaz",
			},
		},
	}

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateInvalidVersion(t *testing.T) {
	config := dict{
		"version": "2.1",
		"services": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	err := Validate(config, "2.1")
	assert.ErrorContains(t, err, "unsupported Compose file version: 2.1")
}

type array []interface{}

func TestValidatePlacement(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"placement": dict{
						"preferences": array{
							dict{
								"spread": "node.labels.az",
							},
						},
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateIsolation(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image":     "busybox",
				"isolation": "some-isolation-value",
			},
		},
	}
	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateRollbackConfig(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"rollback_config": dict{
						"parallelism": 1,
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateRollbackConfigWithOrder(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"rollback_config": dict{
						"parallelism": 1,
						"order":       "start-first",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateRollbackConfigWithUpdateConfig(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"update_config": dict{
						"parallelism": 1,
						"order":       "start-first",
					},
					"rollback_config": dict{
						"parallelism": 1,
						"order":       "start-first",
					},
				},
			},
		},
	}

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}

func TestValidateRollbackConfigWithUpdateConfigFull(t *testing.T) {
	config := dict{
		"version": "3",
		"services": dict{
			"foo": dict{
				"image": "busybox",
				"deploy": dict{
					"update_config": dict{
						"parallelism":    1,
						"order":          "start-first",
						"delay":          "10s",
						"failure_action": "pause",
						"monitor":        "10s",
					},
					"rollback_config": dict{
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

	assert.NilError(t, Validate(config, "3"))
	assert.NilError(t, Validate(config, "3.9"))
}
