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
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestResetRemove(t *testing.T) {
	p, err := Load(types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "(inline)",
				Content: []byte(`
name: test-reset
networks:
  test:
    name: test
    external: true
`),
			},
			{
				Filename: "(override)",
				Content: []byte(`
networks:
  test: !reset {}
`),
			},
		},
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	_, ok := p.Networks["test"]
	assert.Check(t, !ok)
}

func TestOverrideReplace(t *testing.T) {
	p, err := Load(types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "(inline)",
				Content: []byte(`
name: test-override
networks:
  test:
    name: test
    external: true
`),
			},
			{
				Filename: "(override)",
				Content: []byte(`
networks:
  test: !override {}
`),
			},
		},
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	assert.Check(t, p.Networks["test"].External == false)
}

func TestResetCycle(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "simple_alias_no_cycle",
			config: `
name: test
services:
  a: &a
    image: alpine
  a2: *a
`,
			expectError: false,
		},
		{
			name: "simple_alias_reversed_no_cycle",
			config: `
name: test
services:
  a2: &a
    image: alpine
  a: *a
`,
			expectError: false,
		},
		{
			name: "nested_merge_no_cycle",
			config: `
name: test
x-templates:
  x-gluetun: &gluetun
    environment: &gluetun_env
      a: b
  x-gluetun-pia: &gluetun_pia
    <<: *gluetun
  x-gluetun-env-pia: &gluetun_env_pia
    <<: *gluetun_env
  vp0:
    <<: *gluetun_pia
    environment:
      <<: *gluetun_env_pia
`,
			expectError: false,
		},
		{
			name: "multiple_services_common_config",
			config: `
name: test
x-common:
  &common
  restart: unless-stopped

services:
  backend:
    <<: *common
    image: alpine:latest

  backend-static:
    <<: *common
    image: alpine:latest

  backend-worker:
    <<: *common
    image: alpine:latest
`,
			expectError: false,
		},
		{
			name: "direct_self_reference_cycle",
			config: `
name: test
x-healthcheck: &healthcheck
  egress-service:
    <<: *healthcheck
`,
			expectError: true,
			errorMsg:    "cycle detected: node at path x-healthcheck.egress-service.egress-service references node at path x-healthcheck.egress-service",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				_, err := Load(
					types.ConfigDetails{
						ConfigFiles: []types.ConfigFile{
							{
								Filename: "(inline)",
								Content:  []byte(tt.config),
							},
						},
					}, func(options *Options) {
						options.SkipNormalization = true
						options.SkipConsistencyCheck = true
					},
				)

				if tt.expectError {
					assert.Error(t, err, tt.errorMsg)
				} else {
					assert.NilError(t, err)
				}
			},
		)
	}
}
