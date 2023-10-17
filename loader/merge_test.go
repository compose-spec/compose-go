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
	"reflect"
	"testing"

	"github.com/compose-spec/compose-go/consts"
	"github.com/imdario/mergo"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestLoadLogging(t *testing.T) {
	loggingCases := []struct {
		name            string
		loggingBase     string
		loggingOverride string
		expected        *types.LoggingConfig
	}{
		{
			name: "no_override_driver",
			loggingBase: `
name: test-load-logging
services:
  foo:
    logging:
      driver: json-file
      options:
        frequency: 2000
        timeout: 23
`,
			loggingOverride: `
services:
  foo:
    logging:
      driver: json-file
      options:
        timeout: 360
        pretty-print: on
`,
			expected: &types.LoggingConfig{
				Driver: "json-file",
				Options: map[string]string{
					"frequency":    "2000",
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "override_driver",
			loggingBase: `
name: test-load-logging
services:
  foo:
    logging:
      driver: json-file
      options:
        frequency: 2000
        timeout: 23
`,
			loggingOverride: `
services:
  foo:
    logging:
      driver: syslog
      options:
        timeout: 360
        pretty-print: on
`,
			expected: &types.LoggingConfig{
				Driver: "syslog",
				Options: map[string]string{
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "no_base_driver",
			loggingBase: `
name: test-load-logging
services:
  foo:
    logging:
      options:
        frequency: 2000
        timeout: 23
`,
			loggingOverride: `
services:
  foo:
    logging:
      driver: json-file
      options:
        timeout: 360
        pretty-print: on
`,
			expected: &types.LoggingConfig{
				Driver: "json-file",
				Options: map[string]string{
					"frequency":    "2000",
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "no_driver",
			loggingBase: `
name: test-load-logging
services:
  foo:
    logging:
      options:
        frequency: 2000
        timeout: 23
`,
			loggingOverride: `
services:
  foo:
    logging:
      options:
        timeout: 360
        pretty-print: on
`,
			expected: &types.LoggingConfig{
				Options: map[string]string{
					"frequency":    "2000",
					"timeout":      "360",
					"pretty-print": "on",
				},
			},
		},
		{
			name: "no_override_options",
			loggingBase: `
name: test-load-logging
services:
  foo:
    logging:
      driver: json-file
      options:
        frequency: 2000
        timeout: 23
`,
			loggingOverride: `
services:
  foo:
    logging:
      driver: syslog
`,
			expected: &types.LoggingConfig{
				Driver: "syslog",
			},
		},
		{
			name: "no_base",
			loggingBase: `
name: test-load-logging
services:
  foo:
    image: foo
`,
			loggingOverride: `
services:
  foo:
    logging:
      driver: json-file
      options:
        frequency: 2000
`,
			expected: &types.LoggingConfig{
				Driver: "json-file",
				Options: map[string]string{
					"frequency": "2000",
				},
			},
		},
	}

	for _, tc := range loggingCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Content:  []byte(tc.loggingBase),
					},
					{
						Filename: "override.yml",
						Content:  []byte(tc.loggingOverride),
					},
				},
				Environment: types.Mapping{},
			}
			config, err := loadTestProject(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, tc.expected, config.Services[0].Logging)
		})
	}
}

func loadTestProject(configDetails types.ConfigDetails) (*types.Project, error) {
	return Load(configDetails, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
		options.ResolvePaths = false
	})
}

func TestLoadMultipleServicePorts(t *testing.T) {
	portsCases := []struct {
		name         string
		portBase     map[string]interface{}
		portOverride map[string]interface{}
		expected     []types.ServicePortConfig
	}{
		{
			name: "no_override",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"8080:80",
				},
			},
			portOverride: map[string]interface{}{},
			expected: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Published: "8080",
					Target:    80,
					Protocol:  "tcp",
				},
			},
		},
		{
			name: "override_different_published",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"8080:80",
				},
			},
			portOverride: map[string]interface{}{
				"ports": []interface{}{
					"8081:80",
				},
			},
			expected: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Published: "8080",
					Target:    80,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					Published: "8081",
					Target:    80,
					Protocol:  "tcp",
				},
			},
		},
		{
			name: "override_distinct_protocols",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"8080:80/tcp",
				},
			},
			portOverride: map[string]interface{}{
				"ports": []interface{}{
					"8080:80/udp",
				},
			},
			expected: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Published: "8080",
					Target:    80,
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					Published: "8080",
					Target:    80,
					Protocol:  "udp",
				},
			},
		},
		{
			name: "override_one_sided",
			portBase: map[string]interface{}{
				"ports": []interface{}{
					"5000",
					"6000",
				},
			},
			portOverride: map[string]interface{}{},
			expected: []types.ServicePortConfig{
				{
					Mode:     "ingress",
					Target:   5000,
					Protocol: "tcp",
				},
				{
					Mode:     "ingress",
					Target:   6000,
					Protocol: "tcp",
				},
			},
		},
	}

	for _, tc := range portsCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.portBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.portOverride,
							},
						},
					},
				},
			}
			config, err := loadTestProject(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Project{
				Name:       "",
				WorkingDir: "",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Ports:       tc.expected,
						Environment: types.MappingWithEquals{},
						Scale:       1,
					},
				},
			}, config)
		})
	}
}

func TestLoadMultipleSecretsConfig(t *testing.T) {
	portsCases := []struct {
		name           string
		secretBase     map[string]interface{}
		secretOverride map[string]interface{}
		expected       []types.ServiceSecretConfig
	}{
		{
			name: "no_override",
			secretBase: map[string]interface{}{
				"secrets": []interface{}{
					"my_secret",
				},
			},
			secretOverride: map[string]interface{}{},
			expected: []types.ServiceSecretConfig{
				{
					Source: "my_secret",
				},
			},
		},
		{
			name: "override_simple",
			secretBase: map[string]interface{}{
				"secrets": []interface{}{
					"foo_secret",
				},
			},
			secretOverride: map[string]interface{}{
				"secrets": []interface{}{
					"bar_secret",
				},
			},
			expected: []types.ServiceSecretConfig{
				{
					Source: "foo_secret",
				},
				{
					Source: "bar_secret",
				},
			},
		},
		{
			name: "override_same_source",
			secretBase: map[string]interface{}{
				"secrets": []interface{}{
					"foo_secret",
					map[string]interface{}{
						"source": "bar_secret",
						"target": "waw_secret",
					},
				},
			},
			secretOverride: map[string]interface{}{
				"secrets": []interface{}{
					map[string]interface{}{
						"source": "bar_secret",
						"target": "bof_secret",
					},
					map[string]interface{}{
						"source": "baz_secret",
						"target": "waw_secret",
					},
				},
			},
			expected: []types.ServiceSecretConfig{
				{
					Source: "foo_secret",
				},
				{
					Source: "baz_secret",
					Target: "waw_secret",
				},
				{
					Source: "bar_secret",
					Target: "bof_secret",
				},
			},
		},
	}

	for _, tc := range portsCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.secretBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.secretOverride,
							},
						},
					},
				},
			}
			config, err := loadTestProject(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Project{
				Name:       "",
				WorkingDir: "",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Secrets:     tc.expected,
						Environment: types.MappingWithEquals{},
						Scale:       1,
					},
				},
			}, config)
		})
	}
}

func TestLoadMultipleConfigobjsConfig(t *testing.T) {
	portsCases := []struct {
		name           string
		configBase     map[string]interface{}
		configOverride map[string]interface{}
		expected       []types.ServiceConfigObjConfig
	}{
		{
			name: "no_override",
			configBase: map[string]interface{}{
				"configs": []interface{}{
					"my_config",
				},
			},
			configOverride: map[string]interface{}{},
			expected: []types.ServiceConfigObjConfig{
				{
					Source: "my_config",
				},
			},
		},
		{
			name: "override_simple",
			configBase: map[string]interface{}{
				"configs": []interface{}{
					"foo_config",
				},
			},
			configOverride: map[string]interface{}{
				"configs": []interface{}{
					"bar_config",
				},
			},
			expected: []types.ServiceConfigObjConfig{
				{
					Source: "foo_config",
				},
				{
					Source: "bar_config",
				},
			},
		},
		{
			name: "override_same_source",
			configBase: map[string]interface{}{
				"configs": []interface{}{
					"foo_config",
					map[string]interface{}{
						"source": "bar_config",
						"target": "waw_config",
					},
				},
			},
			configOverride: map[string]interface{}{
				"configs": []interface{}{
					map[string]interface{}{
						"source": "bar_config",
						"target": "bof_config",
					},
					map[string]interface{}{
						"source": "baz_config",
						"target": "waw_config",
					},
				},
			},
			expected: []types.ServiceConfigObjConfig{
				{
					Source: "foo_config",
				},
				{
					Source: "baz_config",
					Target: "waw_config",
				},
				{
					Source: "bar_config",
					Target: "bof_config",
				},
			},
		},
	}

	for _, tc := range portsCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.configBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.configOverride,
							},
						},
					},
				},
			}
			config, err := loadTestProject(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Project{
				Name:       "",
				WorkingDir: "",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Configs:     tc.expected,
						Environment: types.MappingWithEquals{},
						Scale:       1,
					},
				},
			}, config)
		})
	}
}

func TestLoadMultipleUlimits(t *testing.T) {
	ulimitCases := []struct {
		name           string
		ulimitBase     map[string]interface{}
		ulimitOverride map[string]interface{}
		expected       map[string]*types.UlimitsConfig
	}{
		{
			name: "no_override",
			ulimitBase: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"noproc": 65535,
				},
			},
			ulimitOverride: map[string]interface{}{},
			expected: map[string]*types.UlimitsConfig{
				"noproc": {
					Single: 65535,
				},
			},
		},
		{
			name: "override_simple",
			ulimitBase: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"noproc": 65535,
				},
			},
			ulimitOverride: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"noproc": 44444,
				},
			},
			expected: map[string]*types.UlimitsConfig{
				"noproc": {
					Single: 44444,
				},
			},
		},
		{
			name: "override_different_notation",
			ulimitBase: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"nofile": map[string]interface{}{
						"soft": 11111,
						"hard": 99999,
					},
					"noproc": 44444,
				},
			},
			ulimitOverride: map[string]interface{}{
				"ulimits": map[string]interface{}{
					"nofile": 55555,
					"noproc": map[string]interface{}{
						"soft": 22222,
						"hard": 33333,
					},
				},
			},
			expected: map[string]*types.UlimitsConfig{
				"noproc": {
					Soft: 22222,
					Hard: 33333,
				},
				"nofile": {
					Single: 55555,
				},
			},
		},
	}

	for _, tc := range ulimitCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.ulimitBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.ulimitOverride,
							},
						},
					},
				},
			}
			config, err := loadTestProject(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Project{
				Name:       "",
				WorkingDir: "",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Ulimits:     tc.expected,
						Environment: types.MappingWithEquals{},
						Scale:       1,
					},
				},
			}, config)
		})
	}
}

func TestLoadMultipleServiceNetworks(t *testing.T) {
	networkCases := []struct {
		name            string
		networkBase     map[string]interface{}
		networkOverride map[string]interface{}
		expected        map[string]*types.ServiceNetworkConfig
	}{
		{
			name: "no_override",
			networkBase: map[string]interface{}{
				"networks": []interface{}{
					"net1",
					"net2",
				},
			},
			networkOverride: map[string]interface{}{},
			expected: map[string]*types.ServiceNetworkConfig{
				"net1": nil,
				"net2": nil,
			},
		},
		{
			name: "override_simple",
			networkBase: map[string]interface{}{
				"networks": []interface{}{
					"net1",
					"net2",
				},
			},
			networkOverride: map[string]interface{}{
				"networks": []interface{}{
					"net1",
					"net3",
				},
			},
			expected: map[string]*types.ServiceNetworkConfig{
				"net1": nil,
				"net2": nil,
				"net3": nil,
			},
		},
		{
			name: "override_with_aliases",
			networkBase: map[string]interface{}{
				"networks": map[string]interface{}{
					"net1": map[string]interface{}{
						"aliases": []interface{}{
							"alias1",
						},
					},
					"net2": nil,
				},
			},
			networkOverride: map[string]interface{}{
				"networks": map[string]interface{}{
					"net1": map[string]interface{}{
						"aliases": []interface{}{
							"alias2",
							"alias3",
						},
					},
					"net3": map[string]interface{}{},
				},
			},
			expected: map[string]*types.ServiceNetworkConfig{
				"net1": {
					Aliases: []string{"alias1", "alias2", "alias3"},
				},
				"net2": nil,
				"net3": {},
			},
		},
	}

	for _, tc := range networkCases {
		t.Run(tc.name, func(t *testing.T) {
			configDetails := types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{
					{
						Filename: "base.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.networkBase,
							},
						},
					},
					{
						Filename: "override.yml",
						Config: map[string]interface{}{
							"services": map[string]interface{}{
								"foo": tc.networkOverride,
							},
						},
					},
				},
			}
			config, err := loadTestProject(configDetails)
			assert.NilError(t, err)
			assert.DeepEqual(t, &types.Project{
				Name:       "",
				WorkingDir: "",
				Services: []types.ServiceConfig{
					{
						Name:        "foo",
						Networks:    tc.expected,
						Environment: types.MappingWithEquals{},
						Scale:       1,
					},
				},
			}, config)
		})
	}
}

func TestLoadMultipleConfigs(t *testing.T) {
	base := `
name: test-load-multiple-configs
services:
  foo:
    image: foo
    entrypoint: echo
    command: hello world
    build:
      context: .
      dockerfile: bar.Dockerfile
    ports:
      - 8080:80
      - 9090:90
    expose:
      - 8080
    labels:
      - foo=bar
    cap_add:
      - NET_ADMIN
`

	override := `
services:
  foo:
    image: baz
    entrypoint: ping
    command: localhost
    build:
      context: .
      dockerfile: foo.Dockerfile
      args:
        - buildno=1
        - password=secret
    ports:
      - 8080:81
    expose:
      - 8080
    labels:
      - foo=baz
    cap_add:
      - SYS_ADMIN
  bar:
    image: bar
`
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Content: []byte(base)},
			{Filename: "override.yml", Content: []byte(override)},
		},
		Environment: map[string]string{},
	}
	config, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	assert.DeepEqual(t, serviceSort(types.Services{
		{
			Name:       "foo",
			Image:      "baz",
			Entrypoint: types.ShellCommand{"ping"},
			Command:    types.ShellCommand{"localhost"},
			Build: &types.BuildConfig{
				Context:    ".",
				Dockerfile: "foo.Dockerfile",
				Args: types.MappingWithEquals{
					"buildno":  strPtr("1"),
					"password": strPtr("secret"),
				},
			},
			Expose: []string{"8080"},
			Ports: []types.ServicePortConfig{
				{
					Mode:      "ingress",
					Target:    80,
					Published: "8080",
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					Target:    90,
					Published: "9090",
					Protocol:  "tcp",
				},
				{
					Mode:      "ingress",
					Target:    81,
					Published: "8080",
					Protocol:  "tcp",
				},
			},
			Labels: types.Labels{
				"foo": "baz",
			},
			CapAdd:      []string{"NET_ADMIN", "SYS_ADMIN"},
			Environment: types.MappingWithEquals{},
			Scale:       1,
		},
		{
			Name:        "bar",
			Image:       "bar",
			Environment: types.MappingWithEquals{},
			Scale:       1,
		},
	}), serviceSort(config.Services))
}

func TestMergeUlimitsConfig(t *testing.T) {
	specials := &specials{
		m: map[reflect.Type]func(dst, src reflect.Value) error{
			reflect.TypeOf(&types.UlimitsConfig{}): mergeUlimitsConfig,
		},
	}
	base := map[string]*types.UlimitsConfig{
		"override-single":                {Single: 100},
		"override-single-with-soft-hard": {Single: 200},
		"override-soft-hard":             {Soft: 300, Hard: 301},
		"override-soft-hard-with-single": {Soft: 400, Hard: 401},
		"dont-override":                  {Single: 500},
	}
	override := map[string]*types.UlimitsConfig{
		"override-single":                {Single: 110},
		"override-single-with-soft-hard": {Soft: 210, Hard: 211},
		"override-soft-hard":             {Soft: 310, Hard: 311},
		"override-soft-hard-with-single": {Single: 410},
		"add":                            {Single: 610},
	}
	err := mergo.Merge(&base, &override, mergo.WithOverride, mergo.WithTransformers(specials))
	assert.NilError(t, err)
	assert.DeepEqual(
		t,
		base,
		map[string]*types.UlimitsConfig{
			"override-single":                {Single: 110},
			"override-single-with-soft-hard": {Soft: 210, Hard: 211},
			"override-soft-hard":             {Soft: 310, Hard: 311},
			"override-soft-hard-with-single": {Single: 410},
			"dont-override":                  {Single: 500},
			"add":                            {Single: 610},
		},
	)
}

func TestInitOverride(t *testing.T) {
	var (
		bt = true
		bf = false
	)
	cases := []struct {
		base     *bool
		override *bool
		expect   bool
	}{
		{
			base:     &bt,
			override: &bf,
			expect:   false,
		},
		{
			base:     nil,
			override: &bt,
			expect:   true,
		},
		{
			base:     &bt,
			override: nil,
			expect:   true,
		},
	}
	for _, test := range cases {
		base := types.ServiceConfig{
			Init: test.base,
		}
		override := types.ServiceConfig{
			Init: test.override,
		}
		config, err := _merge(&base, &override)
		assert.NilError(t, err)
		assert.Check(t, *config.Init == test.expect)
	}
}

func TestMergeServiceNetworkConfig(t *testing.T) {
	base := map[string]*types.ServiceNetworkConfig{
		"override": {
			Aliases:     []string{"100", "101"},
			Ipv4Address: "127.0.0.1",
			Ipv6Address: "0:0:0:0:0:0:0:1",
		},
		"dont-override": {
			Aliases:     []string{"200", "201"},
			Ipv4Address: "127.0.0.2",
			Ipv6Address: "0:0:0:0:0:0:0:2",
		},
	}
	override := map[string]*types.ServiceNetworkConfig{
		"override": {
			Aliases:     []string{"110", "111"},
			Ipv4Address: "127.0.1.1",
			Ipv6Address: "0:0:0:0:0:0:1:1",
		},
		"add": {
			Aliases:     []string{"310", "311"},
			Ipv4Address: "127.0.3.1",
			Ipv6Address: "0:0:0:0:0:0:3:1",
		},
	}
	err := mergo.Merge(&base, &override, mergo.WithAppendSlice, mergo.WithOverride)
	assert.NilError(t, err)
	assert.DeepEqual(
		t,
		base,
		map[string]*types.ServiceNetworkConfig{
			"override": {
				Aliases:     []string{"100", "101", "110", "111"},
				Ipv4Address: "127.0.1.1",
				Ipv6Address: "0:0:0:0:0:0:1:1",
			},
			"dont-override": {
				Aliases:     []string{"200", "201"},
				Ipv4Address: "127.0.0.2",
				Ipv6Address: "0:0:0:0:0:0:0:2",
			},
			"add": {
				Aliases:     []string{"310", "311"},
				Ipv4Address: "127.0.3.1",
				Ipv6Address: "0:0:0:0:0:0:3:1",
			},
		},
	)
}

func TestMergeTopLevelExtensions(t *testing.T) {
	base := map[string]interface{}{
		"x-foo": "foo",
		"x-bar": map[string]interface{}{
			"base": map[string]interface{}{},
		},
	}
	override := map[string]interface{}{
		"x-bar": map[string]interface{}{
			"base": "qix",
		},
		"x-zot": "zot",
	}
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: base},
			{Filename: "override.yml", Config: override},
		},
	}
	config, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	assert.DeepEqual(t, types.Extensions{
		"x-foo": "foo",
		"x-bar": map[string]interface{}{
			"base": "qix",
		},
		"x-zot": "zot",
	}, config.Extensions)
}

func TestMergeCommands(t *testing.T) {
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image":   "alpine",
						"command": "/bin/bash -c \"echo 'hello'\"",
					},
				},
			}},
			{Filename: "override.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image":   "alpine",
						"command": "/bin/ash -c \"echo 'world'\"",
					},
				},
			}},
		},
	}
	merged, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	assert.DeepEqual(t, merged.Services[0].Command, types.ShellCommand{"/bin/ash", "-c", "echo 'world'"})
}

func TestMergeHealthCheck(t *testing.T) {
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image": "alpine",
						"healthcheck": map[string]interface{}{
							"test": []interface{}{"CMD", "original"},
						},
					},
				},
			}},
			{Filename: "override.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image": "alpine",
						"healthcheck": map[string]interface{}{
							"test": []interface{}{"CMD", "override"},
						},
					},
				},
			}},
			{Filename: "override.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image": "alpine",
						"healthcheck": map[string]interface{}{
							"timeout": "30s",
						},
					},
				},
			}},
		},
	}
	merged, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	check := merged.Services[0].HealthCheck
	assert.DeepEqual(t, check.Test, types.HealthCheckTest{"CMD", "override"})
	assert.Equal(t, check.Timeout.String(), "30s")
}

func TestMergeEnvironments(t *testing.T) {
	configDetails := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image": "alpine",
						"environment": map[string]interface{}{
							"NAME":  "BASE",
							"VALUE": "BASE",
						},
					},
				},
			}},
			{Filename: "override.yml", Config: map[string]interface{}{
				"services": map[string]interface{}{
					"foo": map[string]interface{}{
						"image": "alpine",
						"environment": map[string]interface{}{
							"NAME":  "DEV",
							"VALUE": nil,
						},
					},
				},
			}},
		},
	}
	merged, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	env := merged.Services[0].Environment
	assert.Assert(t, *env["NAME"] == "DEV")
	assert.Assert(t, env["VALUE"] == nil)
}

func TestMergeExtraHosts(t *testing.T) {
	base := types.HostsList{
		"kept":              "192.168.1.100",
		"extra1.domain.org": "192.168.1.101",
		"extra2.domain.org": "192.168.1.102",
	}
	override := types.HostsList{
		"extra1.domain.org": "10.0.0.1",
		"extra2.domain.org": "10.0.0.2",
		"added":             "10.0.0.3",
	}
	err := mergo.Merge(&base, &override, mergo.WithOverride)
	assert.NilError(t, err)
	assert.DeepEqual(
		t,
		base,
		types.HostsList{
			"kept":              "192.168.1.100",
			"extra1.domain.org": "10.0.0.1",
			"extra2.domain.org": "10.0.0.2",
			"added":             "10.0.0.3",
		},
	)
}

func TestLoadWithNullOverride(t *testing.T) {
	base := `
name: test
services:
  foo:
    build:
      context: .
      dockerfile: foo.Dockerfile
    read_only: true
    environment:
      FOO: BAR
    ports:
      - "8080:80"
  bar:
    image: test
    ports:
      - "8443:443"

`
	override := `
services:
  foo:
    image: foo
    build: !reset {}
    read_only: !reset false
    environment:
      FOO: !reset
    ports: !reset []
`
	configDetails := types.ConfigDetails{
		Environment: map[string]string{},
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Content: []byte(base)},
			{Filename: "override.yml", Content: []byte(override)},
		},
	}
	config, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	got := serviceSort(config.Services)
	assert.DeepEqual(t, []types.ServiceConfig{
		{
			Name:        "bar",
			Image:       "test",
			Environment: types.MappingWithEquals{},
			Ports:       []types.ServicePortConfig{{Mode: "ingress", Target: 443, Published: "8443", Protocol: "tcp"}},
			Scale:       1,
		},
		{
			Build:       nil,
			Name:        "foo",
			Image:       "foo",
			Environment: types.MappingWithEquals{},
			Ports:       nil,
			ReadOnly:    false,
			Scale:       1,
		},
	}, got)
}

func TestMergeExpose(t *testing.T) {
	base := `
name: test
services:
  foo:
    image: foo
    expose:
      - "8080"
      - "8081"
      - "8082"
      - "8083"
      - "8084"
`
	override := `
services:
  foo:
    image: foo
    expose:
      - "8090"
      - "8091"
      - "8082"
      - "8081"
`
	configDetails := types.ConfigDetails{
		Environment: map[string]string{},
		ConfigFiles: []types.ConfigFile{
			{Filename: "base.yml", Content: []byte(base)},
			{Filename: "override.yml", Content: []byte(override)},
		},
	}
	config, err := loadTestProject(configDetails)
	assert.NilError(t, err)
	assert.DeepEqual(t, &types.Project{
		Name:       "test",
		WorkingDir: "",
		Services: []types.ServiceConfig{
			{
				Name:        "foo",
				Image:       "foo",
				Environment: types.MappingWithEquals{},
				Expose:      types.StringOrNumberList{"8080", "8081", "8082", "8083", "8084", "8090", "8091"},
				Scale:       1,
			},
		},
		Environment: map[string]string{consts.ComposeProjectName: "test"},
	}, config)
}
