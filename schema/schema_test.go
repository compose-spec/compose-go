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
	"testing"

	"gotest.tools/v3/assert"
)

type dict map[string]interface{}
type array []interface{}

type ComposeTestCase struct {
	name    string
	config  dict
	invalid bool
}

func ValidateAll(t *testing.T, tests []ComposeTestCase) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schema.Validate(tt.config)
			if tt.invalid && err == nil {
				t.Fail()
			} else if !tt.invalid {
				assert.NilError(t, err)
			}
		})
	}
}

func createConfig(key string, value interface{}) dict {
	config := dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}
	config[key] = value
	return config
}

func createServiceConfig(key string, value interface{}) dict {
	return dict{
		"services": dict{
			"foo": dict{
				"image": "busybox",
				key:     value,
			},
		},
	}
}

func TestComposeSpecSchemaIsValid(t *testing.T) {
	_, err := CreateComposeSchema()
	assert.NilError(t, err)
}

var schema, _ = CreateComposeSchema()

func TestValidateUndefinedTopLevelOption(t *testing.T) {
	config := dict{
		"helicopters": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	err := schema.Validate(config)
	assert.ErrorContains(t, err, `Additional property helicopters is not allowed`)
}

func TestValidateAllowsXFields(t *testing.T) {
	config := dict{
		"x-test": dict{},
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
	assert.NilError(t, schema.Validate(config))
}

func TestValidateSecretConfigNames(t *testing.T) {
	config := dict{
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

	assert.NilError(t, schema.Validate(config))
}

func TestHealthcheck(t *testing.T) {
	config := createServiceConfig("healthcheck", dict{
		"test":           array{"CMD", "curl", "-f", "http://localhost"},
		"interval":       "1m30s",
		"timeout":        "10s",
		"retries":        3,
		"start_period":   "40s",
		"start_interval": "5s",
	})

	assert.NilError(t, schema.Validate(config))
}

func TestServiceExpose(t *testing.T) {
	tests := []ComposeTestCase{
		{
			name: "[#/services/service/expose] expose port has right format",
			config: createServiceConfig("expose", array{
				"12345-12345/tcp",
				"12345-12345/udp",
				"12345/udp",
				"12345-12345",
				"1234-1234",
				"123-123",
				"12-12",
				"1-1",
				"12345",
				"1234",
				"123",
				"12",
				"1",
			}),
		},
		{
			name:    "[#/services/service/expose] expose port is too long",
			config:  createServiceConfig("expose", array{"123456"}),
			invalid: true,
		},
		{
			name:    "[#/services/service/expose] expose second port is too long",
			config:  createServiceConfig("expose", array{"1234-123456"}),
			invalid: true,
		},
		{
			name:    "[#/services/service/expose] expose missing first port",
			config:  createServiceConfig("expose", array{"-123"}),
			invalid: true,
		},
		{
			name:    "[#/services/service/expose] expose with wrong protocol",
			config:  createServiceConfig("expose", array{"123-123/random"}),
			invalid: true,
		},
	}

	ValidateAll(t, tests)
}
func TestServicePorts(t *testing.T) {
	tests := []ComposeTestCase{
		{
			name: "[#/services/service/ports] ports must have the [HOST:]CONTAINER[/PROTOCOL] format",
			config: createServiceConfig("ports", array{
				"3000",
				"3000-3005",
				"8000:8000",
				"9090-9091:8080-8081",
				"49100:22",
				"8000-9000:80",
				"127.0.0.1:8001:8001",
				"127.0.0.1:5000-5010:5000-5010",
				"6060:6060/udp",
			}),
		},
		{
			name:    "[#/services/service/ports] ports cannot be empty",
			config:  createServiceConfig("ports", array{""}),
			invalid: true,
		},
		{
			name:    "[#/services/service/ports] ports cannot be negative",
			config:  createServiceConfig("ports", array{"-3000"}),
			invalid: true,
		},
		{
			name:    "[#/services/service/ports] ports cannot be incomplete",
			config:  createServiceConfig("ports", array{":80"}),
			invalid: true,
		},
		{
			name:    "[#/services/service/ports] ports must have valid protocol",
			config:  createServiceConfig("ports", array{"6060:6060/random"}),
			invalid: true,
		},
		{
			name: "[#/services/service/ports] port protocol should be tcp or udp",
			config: createServiceConfig("ports", array{
				dict{"protocol": "tcp"},
				dict{"protocol": "udp"},
			}),
		},
		{
			name: "[#/services/service/ports] port protocol should ONLY be tcp or udp",
			config: createServiceConfig("ports", array{
				dict{"protocol": "random"},
			}),
			invalid: true,
		},
		{
			name: "[#/services/service/ports] port published within range",
			config: createServiceConfig("ports", array{
				dict{"published": "0"},
				dict{"published": "00000"},
				dict{"published": "0-0"},
				dict{"published": "00000-00000"},
				dict{"published": "65535"},
				dict{"published": "65535-65535"},
				dict{"published": 0},
				dict{"published": 65535},
			}),
		},
		{
			name:    "[#/services/service/ports] port published should not be over 65535 (integer)",
			config:  createServiceConfig("ports", array{dict{"published": 65536}}),
			invalid: true,
		},
		{
			name:    "[#/services/service/ports] published should not be over 65535 (string)",
			config:  createServiceConfig("ports", array{dict{"published": "65536"}}),
			invalid: true,
		},
		{
			name:    "[#/services/service/ports] port published should not be over 65535 (string)",
			config:  createServiceConfig("ports", array{dict{"published": "65535-65536"}}),
			invalid: true,
		},
	}
	ValidateAll(t, tests)
}

func TestStopGracePeriod(t *testing.T) {
	tests := []ComposeTestCase{
		{
			name:   "[#/services/service/stop_grace_period] must be a valid duration (micro second)",
			config: createServiceConfig("stop_grace_period", "10us"),
		},
		{
			name:   "[#/services/service/stop_grace_period] must be a valid duration (millisecond)",
			config: createServiceConfig("stop_grace_period", "10ms"),
		},
		{
			name:   "[#/services/service/stop_grace_period] must be a valid duration (second)",
			config: createServiceConfig("stop_grace_period", "10s"),
		},
		{
			name:   "[#/services/service/stop_grace_period] must be a valid duration (minute)",
			config: createServiceConfig("stop_grace_period", "10m"),
		},
		{
			name:   "[#/services/service/stop_grace_period] must be a valid duration (hour)",
			config: createServiceConfig("stop_grace_period", "10h"),
		},
		{
			name:   "[#/services/service/stop_grace_period] must be a valid duration (combined)",
			config: createServiceConfig("stop_grace_period", "1h2m3s4ms5us"),
		},
		{
			name:    "[services/service/stop_grace_period] cannot be given a duration with wrong order",
			config:  createServiceConfig("stop_grace_period", "1s10h"),
			invalid: true,
		},
		{
			name:    "stop_grace_period cannot be given a duration with wrong unit",
			config:  createServiceConfig("stop_grace_period", "1kg"),
			invalid: true,
		},
	}
	ValidateAll(t, tests)
}

func TestValidatePlacement(t *testing.T) {
	config := createServiceConfig("deploy", dict{
		"placement": dict{
			"preferences": array{
				dict{
					"spread": "node.labels.az",
				},
			},
		},
	})

	assert.NilError(t, schema.Validate(config))
}

func TestValidateIsolation(t *testing.T) {
	config := createServiceConfig("isolation", "some-isolation-value")

	assert.NilError(t, schema.Validate(config))
}

func TestRollbackConfig(t *testing.T) {
	tests := []ComposeTestCase{
		{
			name: "[#/services/service/deploy/rollback_config] should be valid",
			config: createServiceConfig("deploy", dict{
				"rollback_config": dict{
					"parallelism":    1,
					"order":          "start-first",
					"delay":          "10s",
					"failure_action": "pause",
					"monitor":        "10s",
				},
			}),
		},
		{
			name: "[#/services/service/deploy/rollback_config/monitor] is a duration",
			config: createServiceConfig("deploy", dict{
				"rollback_config": dict{
					"monitor": "1h2s3ms4us5ns",
				},
			}),
		},
		{
			name: "[#/services/service/deploy/rollback_config/delay] is a duration",
			config: createServiceConfig("deploy", dict{
				"rollback_config": dict{
					"delay": "1h2s3ms4us",
				},
			}),
		},
		{
			name: "[#/services/service/deploy] with rollback and update config",
			config: createServiceConfig("deploy", dict{
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
			}),
		},
	}
	ValidateAll(t, tests)
}

func TestInclude(t *testing.T) {
	tests := []ComposeTestCase{
		{
			name:   "[#/services/service/develop/include] should be an object array",
			config: createConfig("include", array{dict{"path": "foo"}}),
		},
		{
			name:   "[#/services/service/develop/include] should be a string array",
			config: createConfig("include", array{"foo"}),
		},
	}
	ValidateAll(t, tests)
}

func TestWatch(t *testing.T) {
	tests := []ComposeTestCase{
		{
			name: "[#/services/service/develop/watch] should require action and path",
			config: createServiceConfig("develop", dict{
				"x-develop": "foo",
				"watch": array{
					dict{
						"action":  "sync",
						"path":    "foo",
						"x-watch": "foo",
					},
				},
			}),
		},
		{
			name: "[services/service/develop/watch] should require at least one element",
			config: createServiceConfig("develop", dict{
				"watch": array{},
			}),
			invalid: true,
		},
		{
			name: "[services/service/develop/watch] should require at least one element with path and action",
			config: createServiceConfig("develop", dict{
				"watch": array{
					dict{
						"action": "rebuild",
					},
				},
			}),
			invalid: true,
		},
	}

	ValidateAll(t, tests)
}
