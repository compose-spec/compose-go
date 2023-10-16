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
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/compose-spec/compose-go/types"
)

func buildConfigDetails(yaml string, env map[string]string) types.ConfigDetails {
	return buildConfigDetailsMultipleFiles(env, yaml)
}

func buildConfigDetailsMultipleFiles(env map[string]string, yamls ...string) types.ConfigDetails {
	workingDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if env == nil {
		env = map[string]string{}
	}

	return types.ConfigDetails{
		WorkingDir:  workingDir,
		ConfigFiles: buildConfigFiles(yamls),
		Environment: env,
	}
}

func buildConfigFiles(yamls []string) []types.ConfigFile {
	configFiles := []types.ConfigFile{}
	for i, yaml := range yamls {
		configFiles = append(configFiles, types.ConfigFile{
			Filename: fmt.Sprintf("filename%d.yml", i),
			Content:  []byte(yaml)})
	}
	return configFiles
}

func loadYAML(yaml string) (*types.Project, error) {
	return loadYAMLWithEnv(yaml, nil)
}

func loadYAMLWithEnv(yaml string, env map[string]string) (*types.Project, error) {
	return Load(buildConfigDetails(yaml, env), func(options *Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
}

var sampleYAML = `
name: sample
services:
  foo:
    image: busybox
    networks:
      with_me:
  bar:
    image: busybox
    environment:
      - FOO=1
    networks:
      - with_ipam
volumes:
  hello:
    driver: default
    driver_opts:
      beep: boop
networks:
  default:
    driver: bridge
    driver_opts:
      beep: boop
  with_ipam:
    ipam:
      driver: default
      config:
        - subnet: 172.28.0.0/16
`

var sampleDict = map[string]interface{}{
	"name": "sample",
	"services": map[string]interface{}{
		"foo": map[string]interface{}{
			"image":    "busybox",
			"networks": map[string]interface{}{"with_me": nil},
		},
		"bar": map[string]interface{}{
			"image":       "busybox",
			"environment": []interface{}{"FOO=1"},
			"networks":    []interface{}{"with_ipam"},
		},
	},
	"volumes": map[string]interface{}{
		"hello": map[string]interface{}{
			"driver": "default",
			"driver_opts": map[string]interface{}{
				"beep": "boop",
			},
		},
	},
	"networks": map[string]interface{}{
		"default": map[string]interface{}{
			"driver": "bridge",
			"driver_opts": map[string]interface{}{
				"beep": "boop",
			},
		},
		"with_ipam": map[string]interface{}{
			"ipam": map[string]interface{}{
				"driver": "default",
				"config": []interface{}{
					map[string]interface{}{
						"subnet": "172.28.0.0/16",
					},
				},
			},
		},
	},
}

var samplePortsConfig = []types.ServicePortConfig{
	{
		Mode:      "ingress",
		Target:    8080,
		Published: "80",
		Protocol:  "tcp",
	},
	{
		Mode:      "ingress",
		Target:    8081,
		Published: "81",
		Protocol:  "tcp",
	},
	{
		Mode:      "ingress",
		Target:    8082,
		Published: "82",
		Protocol:  "tcp",
	},
	{
		Mode:      "ingress",
		Target:    8090,
		Published: "90",
		Protocol:  "udp",
	},
	{
		Mode:      "ingress",
		Target:    8091,
		Published: "91",
		Protocol:  "udp",
	},
	{
		Mode:      "ingress",
		Target:    8092,
		Published: "92",
		Protocol:  "udp",
	},
	{
		Mode:      "ingress",
		Target:    8500,
		Published: "85",
		Protocol:  "tcp",
	},
	{
		Mode:     "ingress",
		Target:   8600,
		Protocol: "tcp",
	},
	{
		Target:    53,
		Published: "10053",
		Protocol:  "udp",
	},
	{
		Mode:      "host",
		Target:    22,
		Published: "10022",
	},
}

func strPtr(val string) *string {
	return &val
}

var sampleConfig = types.Config{
	Services: []types.ServiceConfig{
		{
			Name:        "foo",
			Image:       "busybox",
			Environment: map[string]*string{},
			Networks: map[string]*types.ServiceNetworkConfig{
				"with_me": nil,
			},
			Scale: 1,
		},
		{
			Name:        "bar",
			Image:       "busybox",
			Environment: map[string]*string{"FOO": strPtr("1")},
			Networks: map[string]*types.ServiceNetworkConfig{
				"with_ipam": nil,
			},
			Scale: 1,
		},
	},
	Networks: map[string]types.NetworkConfig{
		"default": {
			Driver: "bridge",
			DriverOpts: map[string]string{
				"beep": "boop",
			},
		},
		"with_ipam": {
			Ipam: types.IPAMConfig{
				Driver: "default",
				Config: []*types.IPAMPool{
					{
						Subnet: "172.28.0.0/16",
					},
				},
			},
		},
	},
	Volumes: map[string]types.VolumeConfig{
		"hello": {
			Driver: "default",
			DriverOpts: map[string]string{
				"beep": "boop",
			},
		},
	},
}

func TestParseYAML(t *testing.T) {
	dict, err := ParseYAML([]byte(sampleYAML))
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(sampleDict, dict))
}

func TestLoad(t *testing.T) {
	actual, err := Load(buildConfigDetails(sampleYAML, nil), func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(serviceSort(sampleConfig.Services), serviceSort(actual.Services)))
	assert.Check(t, is.DeepEqual(sampleConfig.Networks, actual.Networks))
	assert.Check(t, is.DeepEqual(sampleConfig.Volumes, actual.Volumes))
}

func TestLoadFromFile(t *testing.T) {
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd(): %s", err)
	}
	tmpdir := t.TempDir()
	tmpPath := filepath.Join(tmpdir, "Docker-compose.yaml")
	if err := os.WriteFile(tmpPath, []byte(sampleYAML), 0o444); err != nil {
		t.Fatalf("failed to write temporary file: %s", err)
	}
	actual, err := Load(types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{{
			Filename: tmpPath,
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(serviceSort(sampleConfig.Services), serviceSort(actual.Services)))
	assert.Check(t, is.DeepEqual(sampleConfig.Networks, actual.Networks))
	assert.Check(t, is.DeepEqual(sampleConfig.Volumes, actual.Volumes))
}

func TestLoadExtensions(t *testing.T) {
	actual, err := loadYAML(`
name: load-extensions
services:
  foo:
    image: busybox
    x-foo: bar`)
	assert.NilError(t, err)
	assert.Check(t, is.Len(actual.Services, 1))
	service := actual.Services[0]
	assert.Check(t, is.Equal("busybox", service.Image))
	extras := types.Extensions{
		"x-foo": "bar",
	}
	assert.Check(t, is.DeepEqual(extras, service.Extensions))
}

func TestLoadExtends(t *testing.T) {
	actual, err := loadYAML(`
name: load-extends
services:
  foo:
    image: busybox
    extends:
      service: bar
  bar:
    image: alpine
    command: echo`)
	assert.NilError(t, err)
	assert.Check(t, is.Len(actual.Services, 2))
	service, err := actual.GetService("foo")
	assert.NilError(t, err)
	assert.Check(t, service.Image == "busybox")
	assert.Check(t, service.Command[0] == "echo")
}

func TestLoadExtendsOverrideCommand(t *testing.T) {
	actual, err := loadYAML(`
name: override-command
services:
  foo:
    image: busybox
    extends:
      service: bar
    command: "/bin/ash -c \"rm -rf /tmp/might-not-exist\""
  bar:
    image: alpine
    command: "/bin/ash -c \"echo Oh no...\""`)
	assert.NilError(t, err)
	assert.Check(t, is.Len(actual.Services, 2))
	service, err := actual.GetService("foo")
	assert.NilError(t, err)
	assert.Check(t, service.Image == "busybox")
	assert.DeepEqual(t, service.Command, types.ShellCommand{"/bin/ash", "-c", "rm -rf /tmp/might-not-exist"})
}

func TestLoadExtendsMultipleFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Test creates real files on disk")
	}

	tmpdir := t.TempDir()

	aDir := filepath.Join(tmpdir, "a")
	assert.NilError(t, os.Mkdir(aDir, 0o700))
	aYAML := `
services:
  a:
    build: .
`
	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "a", "compose.yaml"), []byte(aYAML), 0o600))

	bDir := filepath.Join(tmpdir, "b")
	assert.NilError(t, os.Mkdir(bDir, 0o700))
	bYAML := `
services:
  b:
    build:
      target: fake
`
	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "b", "compose.yaml"), []byte(bYAML), 0o600))

	rootYAML := `
services:
  a:
    extends:
      file: ./a/compose.yaml
      service: a
  b:
    extends:
      file: ./b/compose.yaml
      service: b
`
	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "compose.yaml"), []byte(rootYAML), 0o600))

	actual, err := Load(types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(tmpdir, "compose.yaml"),
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(actual.Services, 2))

	svcA, err := actual.GetService("a")
	assert.NilError(t, err)
	assert.Equal(t, svcA.Build.Context, aDir)

	svcB, err := actual.GetService("b")
	assert.NilError(t, err)
	assert.Equal(t, svcB.Build.Context, bDir)
}

func TestLoadExtendsWihReset(t *testing.T) {
	actual, err := loadYAML(`
name: load-extends
services:
  foo:
    extends:
      service: bar
    volumes: !reset []
  bar:
    image: alpine
    command: echo
    volumes:
       - .:/src
`)
	assert.NilError(t, err)
	assert.Check(t, is.Len(actual.Services, 2))
	foo, err := actual.GetService("foo")
	assert.NilError(t, err)
	assert.Check(t, len(foo.Volumes) == 0)
}

func TestLoadCredentialSpec(t *testing.T) {
	actual, err := loadYAML(`
name: load-credential-spec
services:
  foo:
    image: busybox
    credential_spec:
      config: "0bt9dmxjvjiqermk6xrop3ekq"
`)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(actual.Services, 1))
	assert.Check(t, is.Equal(actual.Services[0].CredentialSpec.Config, "0bt9dmxjvjiqermk6xrop3ekq"))
}

func TestParseAndLoad(t *testing.T) {
	actual, err := loadYAML(sampleYAML)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(serviceSort(sampleConfig.Services), serviceSort(actual.Services)))
	assert.Check(t, is.DeepEqual(sampleConfig.Networks, actual.Networks))
	assert.Check(t, is.DeepEqual(sampleConfig.Volumes, actual.Volumes))
}

func TestInvalidTopLevelObjectType(t *testing.T) {
	_, err := loadYAML("1")
	assert.ErrorContains(t, err, "Top-level object must be a mapping")

	_, err = loadYAML("\"hello\"")
	assert.ErrorContains(t, err, "Top-level object must be a mapping")

	_, err = loadYAML("[\"hello\"]")
	assert.ErrorContains(t, err, "Top-level object must be a mapping")
}

func TestNonStringKeys(t *testing.T) {
	_, err := loadYAML(`
123:
  foo:
    image: busybox
`)
	assert.ErrorContains(t, err, "Non-string key at top level: 123")

	_, err = loadYAML(`
services:
  foo:
    image: busybox
  123:
    image: busybox
`)
	assert.ErrorContains(t, err, "Non-string key in services: 123")

	_, err = loadYAML(`
services:
  foo:
    image: busybox
networks:
  default:
    ipam:
      config:
        - 123: oh dear
`)
	assert.ErrorContains(t, err, "Non-string key in networks.default.ipam.config[0]: 123")

	_, err = loadYAML(`
services:
  dict-env:
    image: busybox
    environment:
      1: FOO
`)
	assert.ErrorContains(t, err, "Non-string key in services.dict-env.environment: 1")
}

func TestV1Unsupported(t *testing.T) {
	_, err := loadYAML(`
foo:
  image: busybox
`)
	assert.Check(t, err != nil)
}

func TestNonMappingObject(t *testing.T) {
	_, err := loadYAML(`
name: non-mapping-object
services:
  - foo:
      image: busybox
`)
	assert.ErrorContains(t, err, "services must be a mapping")

	_, err = loadYAML(`
name: non-mapping-object
services:
  foo: busybox
`)
	assert.ErrorContains(t, err, "services.foo must be a mapping")

	_, err = loadYAML(`
name: non-mapping-object
networks:
  - default:
      driver: bridge
`)
	assert.ErrorContains(t, err, "networks must be a mapping")

	_, err = loadYAML(`
name: non-mapping-object
networks:
  default: bridge
`)
	assert.ErrorContains(t, err, "networks.default must be a mapping")

	_, err = loadYAML(`
name: non-mapping-object
volumes:
  - data:
      driver: local
`)
	assert.ErrorContains(t, err, "volumes must be a mapping")

	_, err = loadYAML(`
name: non-mapping-object
volumes:
  data: local
`)
	assert.ErrorContains(t, err, "volumes.data must be a mapping")
}

func TestNonStringImage(t *testing.T) {
	_, err := loadYAML(`
name: non-mapping-object
services:
  foo:
    image: ["busybox", "latest"]
`)
	assert.ErrorContains(t, err, "services.foo.image must be a string")
}

func TestLoadWithEnvironment(t *testing.T) {
	config, err := loadYAMLWithEnv(`
name: load-with-environment
services:
  dict-env:
    image: busybox
    environment:
      FOO: "1"
      BAR: 2
      GA: 2.5
      BU: ""
      ZO:
      MEU:
  list-env:
    image: busybox
    environment:
      - FOO=1
      - BAR=2
      - GA=2.5
      - BU=
      - ZO
      - MEU
`, map[string]string{"MEU": "Shadoks"})
	assert.NilError(t, err)

	expected := types.MappingWithEquals{
		"FOO": strPtr("1"),
		"BAR": strPtr("2"),
		"GA":  strPtr("2.5"),
		"BU":  strPtr(""),
		"ZO":  nil,
		"MEU": strPtr("Shadoks"),
	}

	assert.Check(t, is.Equal(2, len(config.Services)))

	for _, service := range config.Services {
		assert.Check(t, is.DeepEqual(expected, service.Environment))
	}
}

func TestLoadEnvironmentWithBoolean(t *testing.T) {
	config, err := loadYAML(`
name: load-environment-with-boolean
services:
  dict-env:
    image: busybox
    environment:
      FOO: true
      BAR: false
`)
	assert.NilError(t, err)

	expected := types.MappingWithEquals{
		"FOO": strPtr("true"),
		"BAR": strPtr("false"),
	}

	assert.Check(t, is.Equal(1, len(config.Services)))

	for _, service := range config.Services {
		assert.Check(t, is.DeepEqual(expected, service.Environment))
	}
}

func TestInvalidEnvironmentValue(t *testing.T) {
	_, err := loadYAML(`
name: invalid-environment-value
services:
  dict-env:
    image: busybox
    environment:
      FOO: ["1"]
`)
	assert.ErrorContains(t, err, "services.dict-env.environment.FOO must be a string, number, boolean or null")
}

func TestInvalidEnvironmentObject(t *testing.T) {
	_, err := loadYAML(`
name: invalid-environment-object
services:
  dict-env:
    image: busybox
    environment: "FOO=1"
`)
	assert.ErrorContains(t, err, "services.dict-env.environment must be a mapping")
}

func TestLoadWithEnvironmentInterpolation(t *testing.T) {
	home := "/home/foo"
	config, err := loadYAMLWithEnv(`
# This is a comment, so using variable syntax here ${SHOULD_NOT_BREAK} parsing
name: load-with-environment-interpolation
services:
  test:
    image: busybox
    labels:
      - home1=$HOME
      - home2=${HOME}
      - nonexistent=$NONEXISTENT
      - default=${NONEXISTENT-default}
networks:
  test:
    driver: $HOME
volumes:
  test:
    driver: $HOME
`, map[string]string{
		"HOME": home,
		"FOO":  "foo",
	})

	assert.NilError(t, err)

	expectedLabels := types.Labels{
		"home1":       home,
		"home2":       home,
		"nonexistent": "",
		"default":     "default",
	}

	assert.Check(t, is.DeepEqual(expectedLabels, config.Services[0].Labels))
	assert.Check(t, is.Equal(home, config.Networks["test"].Driver))
	assert.Check(t, is.Equal(home, config.Volumes["test"].Driver))
}

func TestLoadWithInterpolationCastFull(t *testing.T) {
	dict := `
name: load-with-interpolation-cast-full
services:
  web:
    configs:
      - source: appconfig
        mode: $theint
    secrets:
      - source: super
        mode: $theint
    healthcheck:
      retries: ${theint}
      disable: $thebool
    deploy:
      replicas: $theint
      update_config:
        parallelism: $theint
        max_failure_ratio: $thefloat
      rollback_config:
        parallelism: $theint
        max_failure_ratio: $thefloat
      restart_policy:
        max_attempts: $theint
      placement:
        max_replicas_per_node: $theint
    ports:
      - $theint
      - "34567"
      - target: $theint
        published: "$theint"
        x-foo-bar: true
    ulimits:
      nproc: $theint
      nofile:
        hard: $theint
        soft: $theint
    privileged: $thebool
    read_only: $thebool
    shm_size: ${thesize}
    stop_grace_period: ${theduration}
    stdin_open: ${thebool}
    tty: $thebool
    volumes:
      - source: data
        type: volume
        target: /data
        read_only: $thebool
        volume:
          nocopy: $thebool

configs:
  appconfig:
    external: $thebool
secrets:
  super:
    external: $thebool
volumes:
  data:
    external: $thebool
networks:
  front:
    external: $thebool
    internal: $thebool
    attachable: $thebool
  back:
`
	env := map[string]string{
		"theint":      "555",
		"thefloat":    "3.14",
		"thebool":     "true",
		"theduration": "60s",
		"thesize":     "2gb",
	}

	config, err := Load(buildConfigDetails(dict, env), func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
		options.SetProjectName("test", true)
	})
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	duration, err := time.ParseDuration("60s")
	assert.NilError(t, err)
	typesDuration := types.Duration(duration)
	expected := &types.Project{
		Name:        "load-with-interpolation-cast-full",
		Environment: env,
		WorkingDir:  workingDir,
		Services: []types.ServiceConfig{
			{
				Name: "web",
				Configs: []types.ServiceConfigObjConfig{
					{
						Source: "appconfig",
						Mode:   uint32Ptr(555),
					},
				},
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "super",
						Mode:   uint32Ptr(555),
					},
				},
				HealthCheck: &types.HealthCheckConfig{
					Retries: uint64Ptr(555),
					Disable: true,
				},
				Deploy: &types.DeployConfig{
					Replicas: uint64Ptr(555),
					UpdateConfig: &types.UpdateConfig{
						Parallelism:     uint64Ptr(555),
						MaxFailureRatio: 3.14,
					},
					RollbackConfig: &types.UpdateConfig{
						Parallelism:     uint64Ptr(555),
						MaxFailureRatio: 3.14,
					},
					RestartPolicy: &types.RestartPolicy{
						MaxAttempts: uint64Ptr(555),
					},
					Placement: types.Placement{
						MaxReplicas: 555,
					},
				},
				Ports: []types.ServicePortConfig{
					{Target: 555, Mode: "ingress", Protocol: "tcp"},
					{Target: 34567, Mode: "ingress", Protocol: "tcp"},
					{Target: 555, Published: "555", Extensions: map[string]interface{}{"x-foo-bar": true}},
				},
				Ulimits: map[string]*types.UlimitsConfig{
					"nproc":  {Single: 555},
					"nofile": {Hard: 555, Soft: 555},
				},
				Privileged:      true,
				ReadOnly:        true,
				Scale:           1,
				ShmSize:         types.UnitBytes(2 * 1024 * 1024 * 1024),
				StopGracePeriod: &typesDuration,
				StdinOpen:       true,
				Tty:             true,
				Volumes: []types.ServiceVolumeConfig{
					{
						Source:   "data",
						Type:     "volume",
						Target:   "/data",
						ReadOnly: true,
						Volume:   &types.ServiceVolumeVolume{NoCopy: true},
					},
				},
				Environment: types.MappingWithEquals{},
			},
		},
		Configs: map[string]types.ConfigObjConfig{
			"appconfig": {External: types.External{External: true}, Name: "appconfig"},
		},
		Secrets: map[string]types.SecretConfig{
			"super": {External: types.External{External: true}, Name: "super"},
		},
		Volumes: map[string]types.VolumeConfig{
			"data": {External: types.External{External: true}, Name: "data"},
		},
		Networks: map[string]types.NetworkConfig{
			"back": {},
			"front": {
				External:   types.External{External: true},
				Name:       "front",
				Internal:   true,
				Attachable: true,
			},
		},
	}

	assert.Check(t, is.DeepEqual(expected, config))
}

func TestUnsupportedProperties(t *testing.T) {
	dict := `
name: test
services:
  web:
    image: web
    build:
     context: ./web
    links:
      - db
    pid: host
  db:
    image: db
    build:
     context: ./db
`
	configDetails := buildConfigDetails(dict, nil)

	_, err := Load(configDetails)
	assert.NilError(t, err)
}

func TestDiscardEnvFileOption(t *testing.T) {
	dict := `name: test
services:
  web:
    image: nginx
    env_file:
     - example1.env
     - example2.env
`
	expectedEnvironmentMap := types.MappingWithEquals{
		"FOO":                 strPtr("foo_from_env_file"),
		"BAZ":                 strPtr("baz_from_env_file"),
		"BAR":                 strPtr("bar_from_env_file_2"), // Original value is overwritten by example2.env
		"QUX":                 strPtr("quz_from_env_file_2"),
		"ENV.WITH.DOT":        strPtr("ok"),
		"ENV_WITH_UNDERSCORE": strPtr("ok"),
	}
	configDetails := buildConfigDetails(dict, nil)

	// Default behavior keeps the `env_file` entries
	configWithEnvFiles, err := Load(configDetails, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, configWithEnvFiles.Services[0].EnvFile, types.StringList{"example1.env",
		"example2.env"})
	assert.DeepEqual(t, configWithEnvFiles.Services[0].Environment, expectedEnvironmentMap)

	// Custom behavior removes the `env_file` entries
	configWithoutEnvFiles, err := Load(configDetails, WithDiscardEnvFiles)
	assert.NilError(t, err)
	assert.DeepEqual(t, configWithoutEnvFiles.Services[0].EnvFile, types.StringList(nil))
	assert.DeepEqual(t, configWithoutEnvFiles.Services[0].Environment, expectedEnvironmentMap)
}

func TestBuildProperties(t *testing.T) {
	dict := `
name: test
services:
  web:
    image: web
    build: .
    links:
      - db
  db:
    image: db
    build:
     context: ./db
`
	configDetails := buildConfigDetails(dict, nil)
	_, err := Load(configDetails)
	assert.NilError(t, err)
}

func TestDeprecatedProperties(t *testing.T) {
	dict := `
name: test
services:
  web:
    image: web
    container_name: web
  db:
    image: db
    container_name: db
    expose: ["5434"]
`
	configDetails := buildConfigDetails(dict, nil)

	_, err := Load(configDetails)
	assert.NilError(t, err)
}

func TestInvalidResource(t *testing.T) {
	_, err := loadYAML(`
        name: test
        services:
          foo:
            image: busybox
            deploy:
              resources:
                impossible:
                  x: 1
`)
	assert.ErrorContains(t, err, "Additional property impossible is not allowed")
}

func TestInvalidExternalAndDriverCombination(t *testing.T) {
	_, err := loadYAML(`
name: invalid-external-and-driver-combination
volumes:
  external_volume:
    external: true
    driver: foobar
`)

	assert.ErrorContains(t, err, `volumes.external_volume: conflicting parameters "external" and "driver" specified`)
}

func TestInvalidExternalAndDirverOptsCombination(t *testing.T) {
	_, err := loadYAML(`
name: invalid-external-and-driver-opts-combination
volumes:
  external_volume:
    external: true
    driver_opts:
      beep: boop
`)

	assert.ErrorContains(t, err, `volumes.external_volume: conflicting parameters "external" and "driver_opts" specified`)
}

func TestInvalidExternalAndLabelsCombination(t *testing.T) {
	_, err := loadYAML(`
name: invalid-external-and-labels-combination
volumes:
  external_volume:
    external: true
    labels:
      - beep=boop
`)

	assert.ErrorContains(t, err, `volumes.external_volume: conflicting parameters "external" and "labels" specified`)
}

func TestLoadVolumeInvalidExternalNameAndNameCombination(t *testing.T) {
	_, err := loadYAML(`
name: invalid-external-and-labels-combination
volumes:
  external_volume:
    name: user_specified_name
    external:
      name: external_name
`)

	assert.ErrorContains(t, err, "volumes.external_volume: name and external.name conflict; only use name")
}

func TestInterpolateInt(t *testing.T) {
	project, err := loadYAMLWithEnv(`
name: interpolate-int
services:
  foo:
    image: foo
    scale: ${FOO_SCALE}
`, map[string]string{"FOO_SCALE": "2"})

	assert.NilError(t, err)
	assert.Equal(t, project.Services[0].Scale, 2)
}

func durationPtr(value time.Duration) *types.Duration {
	result := types.Duration(value)
	return &result
}

func uint64Ptr(value uint64) *uint64 {
	return &value
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

func TestFullExample(t *testing.T) {
	b, err := os.ReadFile("full-example.yml")
	assert.NilError(t, err)

	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)
	env := map[string]string{
		"HOME": homeDir,
		"BAR":  "this is a secret",
		"QUX":  "qux_from_environment",
	}
	config, err := loadYAMLWithEnv(string(b), env)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	expectedConfig := fullExampleProject(workingDir, homeDir)

	assert.Check(t, is.DeepEqual(expectedConfig.Name, config.Name))
	assert.Check(t, is.DeepEqual(serviceSort(expectedConfig.Services), serviceSort(config.Services)))
	assert.Check(t, is.DeepEqual(expectedConfig.Networks, config.Networks))
	assert.Check(t, is.DeepEqual(expectedConfig.Volumes, config.Volumes))
	assert.Check(t, is.DeepEqual(expectedConfig.Secrets, config.Secrets))
	assert.Check(t, is.DeepEqual(expectedConfig.Configs, config.Configs))
	assert.Check(t, is.DeepEqual(expectedConfig.Extensions, config.Extensions))
}

func TestLoadTmpfsVolume(t *testing.T) {
	for _, size := range []string{"1kb", "1024"} {
		yaml := fmt.Sprintf(`
name: load-tmpfs-volume
services:
  tmpfs:
    image: nginx:latest
    volumes:
    - type: tmpfs
      target: /app
      tmpfs:
        size: %s
`, size)
		config, err := loadYAML(yaml)
		assert.NilError(t, err)

		expected := types.ServiceVolumeConfig{
			Target: "/app",
			Type:   "tmpfs",
			Tmpfs: &types.ServiceVolumeTmpfs{
				Size: types.UnitBytes(1024),
			},
		}

		assert.Assert(t, is.Len(config.Services, 1))
		assert.Check(t, is.Len(config.Services[0].Volumes, 1))
		assert.Check(t, is.DeepEqual(expected, config.Services[0].Volumes[0]))
	}
}

func TestLoadTmpfsVolumeAdditionalPropertyNotAllowed(t *testing.T) {
	_, err := loadYAML(`
name: load-tmpfs-volume-additional-property-not-allowed
services:
  tmpfs:
    image: nginx:latest
    volumes:
      - type: tmpfs
        target: /app
        foo:
          bar: zot
`)
	assert.ErrorContains(t, err, "services.tmpfs.volumes.0 Additional property foo is not allowed")
}

func TestLoadBindMountSourceMustNotBeEmpty(t *testing.T) {
	_, err := loadYAML(`
name: load-bind-mount-source-must-not-be-empty
services:
  tmpfs:
    image: nginx:latest
    volumes:
      - type: bind
        target: /app
`)
	assert.Error(t, err, `invalid mount config for type "bind": field Source must not be empty`)
}

func TestLoadBindMountSourceIsWindowsAbsolute(t *testing.T) {
	tests := []struct {
		doc      string
		yaml     string
		expected types.ServiceVolumeConfig
	}{
		{
			doc: "Z-drive lowercase",
			yaml: `
name: load-bind-mount-source-is-windows-absolute
services:
  windows:
    image: mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019
    volumes:
      - type: bind
        source: z:\
        target: c:\data
`,
			expected: types.ServiceVolumeConfig{Type: "bind", Source: `z:\`, Target: `c:\data`},
		},
		{
			doc: "Z-drive uppercase",
			yaml: `
name: load-bind-mount-source-is-windows-absolute
services:
  windows:
    image: mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019
    volumes:
      - type: bind
        source: Z:\
        target: C:\data
`,
			expected: types.ServiceVolumeConfig{Type: "bind", Source: `Z:\`, Target: `C:\data`},
		},
		{
			doc: "Z-drive subdirectory",
			yaml: `
name: load-bind-mount-source-is-windows-absolute
services:
  windows:
    image: mcr.microsoft.com/windows/servercore/iis:windowsservercore-ltsc2019
    volumes:
      - type: bind
        source: Z:\some-dir
        target: C:\data
`,
			expected: types.ServiceVolumeConfig{Type: "bind", Source: `Z:\some-dir`, Target: `C:\data`},
		},
		{
			doc: "forward-slashes",
			yaml: `
name: load-bind-mount-source-is-windows-absolute
services:
  app:
    image: app:latest
    volumes:
      - type: bind
        source: /z/some-dir
        target: /c/data
`,
			expected: types.ServiceVolumeConfig{Type: "bind", Source: `/z/some-dir`, Target: `/c/data`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			config, err := loadYAML(tc.yaml)
			assert.NilError(t, err)
			assert.Check(t, is.Len(config.Services[0].Volumes, 1))
			assert.Check(t, is.DeepEqual(tc.expected, config.Services[0].Volumes[0]))
		})
	}
}

func TestLoadBindMountWithSource(t *testing.T) {
	config, err := loadYAML(`
name: load-bind-mount-with-source
services:
  bind:
    image: nginx:latest
    volumes:
      - type: bind
        target: /app
        source: "."
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	expected := types.ServiceVolumeConfig{
		Type:   "bind",
		Source: workingDir,
		Target: "/app",
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.Len(config.Services[0].Volumes, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].Volumes[0]))
}

func TestLoadTmpfsVolumeSizeCanBeZero(t *testing.T) {
	config, err := loadYAML(`
name: load-tmpfs-volume-size-can-be-zero
services:
  tmpfs:
    image: nginx:latest
    volumes:
      - type: tmpfs
        target: /app
        tmpfs:
          size: 0
`)
	assert.NilError(t, err)

	expected := types.ServiceVolumeConfig{
		Target: "/app",
		Type:   "tmpfs",
		Tmpfs:  &types.ServiceVolumeTmpfs{},
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.Len(config.Services[0].Volumes, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].Volumes[0]))
}

func TestLoadTmpfsVolumeSizeMustBeGTEQZero(t *testing.T) {
	_, err := loadYAML(`
name: load-tmpfs-volume-size-must-be-gt-eq-zero
services:
  tmpfs:
    image: nginx:latest
    volumes:
      - type: tmpfs
        target: /app
        tmpfs:
          size: -1
`)
	assert.ErrorContains(t, err, "services.tmpfs.volumes.0.tmpfs.size Must be greater than or equal to 0")
}

func TestLoadTmpfsVolumeSizeMustBeInteger(t *testing.T) {
	_, err := loadYAML(`
name: tmpfs-volume-size-must-be-integer
services:
  tmpfs:
    image: nginx:latest
    volumes:
      - type: tmpfs
        target: /app
        tmpfs:
          size: 0.0001
`)
	assert.ErrorContains(t, err, "services.tmpfs.volumes.0.tmpfs.size must be a integer")
}

func serviceSort(services []types.ServiceConfig) []types.ServiceConfig {
	sort.Slice(services, func(i, j int) bool {
		return services[i].Name < services[j].Name
	})
	return services
}

func TestLoadAttachableNetwork(t *testing.T) {
	config, err := loadYAML(`
name: load-attachable-network
networks:
  mynet1:
    driver: overlay
    attachable: true
  mynet2:
    driver: bridge
`)
	assert.NilError(t, err)

	expected := types.Networks{
		"mynet1": {
			Driver:     "overlay",
			Attachable: true,
		},
		"mynet2": {
			Driver:     "bridge",
			Attachable: false,
		},
	}

	assert.Check(t, is.DeepEqual(expected, config.Networks))
}

func TestLoadExpandedPortFormat(t *testing.T) {
	config, err := loadYAML(`
name: load-expanded-port-format
services:
  web:
    image: busybox
    ports:
      - "80-82:8080-8082"
      - "90-92:8090-8092/udp"
      - "85:8500"
      - 8600
      - protocol: udp
        target: 53
        published: 10053
      - mode: host
        target: 22
        published: 10022
`)
	assert.NilError(t, err)

	assert.Check(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(samplePortsConfig, config.Services[0].Ports))
}

func TestLoadExpandedMountFormat(t *testing.T) {
	config, err := loadYAML(`
name: load-expanded-mount-format
services:
  web:
    image: busybox
    volumes:
      - type: volume
        source: foo
        target: /target
        read_only: true
volumes:
  foo: {}
`)
	assert.NilError(t, err)

	expected := types.ServiceVolumeConfig{
		Type:     "volume",
		Source:   "foo",
		Target:   "/target",
		ReadOnly: true,
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.Len(config.Services[0].Volumes, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].Volumes[0]))
}

func TestLoadExtraHostsMap(t *testing.T) {
	config, err := loadYAML(`
name: load-extra-hosts-map
services:
  web:
    image: busybox
    extra_hosts:
      "zulu": "162.242.195.82"
      "alpha": "50.31.209.229"
`)
	assert.NilError(t, err)

	expected := types.HostsList{
		"alpha": "50.31.209.229",
		"zulu":  "162.242.195.82",
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].ExtraHosts))
}

func TestLoadExtraHostsList(t *testing.T) {
	config, err := loadYAML(`
name: load-extra-hosts-list
services:
  web:
    image: busybox
    extra_hosts:
      - "alpha:50.31.209.229"
      - "zulu:ff02::1"
`)
	assert.NilError(t, err)

	expected := types.HostsList{
		"alpha": "50.31.209.229",
		"zulu":  "ff02::1",
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].ExtraHosts))
}

func TestLoadVolumesWarnOnDeprecatedExternalName(t *testing.T) {
	buf, cleanup := patchLogrus()
	defer cleanup()

	project, err := loadYAML(`
name: test-warn-on-deprecated-external-name
volumes:
  foo:
    external:
      name: oops
`)
	assert.NilError(t, err)
	expected := types.Volumes{
		"foo": {
			Name:     "oops",
			External: types.External{External: true},
		},
	}
	assert.Check(t, is.DeepEqual(expected, project.Volumes))
	assert.Check(t, is.Contains(buf.String(), "volumes.foo: external.name is deprecated. Please set name and external: true"))
}

func patchLogrus() (*bytes.Buffer, func()) {
	buf := new(bytes.Buffer)
	out := logrus.StandardLogger().Out
	logrus.SetOutput(buf)
	return buf, func() { logrus.SetOutput(out) }
}

func TestLoadInvalidIsolation(t *testing.T) {
	// validation should be done only on the daemon side
	actual, err := loadYAML(`
name: load-invalid-isolation
services:
  foo:
    image: busybox
    isolation: invalid
configs:
  super:
    external: true
`)
	assert.NilError(t, err)
	assert.Assert(t, is.Len(actual.Services, 1))
	assert.Check(t, is.Equal("invalid", actual.Services[0].Isolation))
}

func TestLoadSecretInvalidExternalNameAndNameCombination(t *testing.T) {
	_, err := loadYAML(`
name: load-secret-invalid-external-name-and-name-combination
secrets:
  external_secret:
    name: user_specified_name
    external:
      name: external_name
`)

	assert.ErrorContains(t, err, "secrets.external_secret: name and external.name conflict; only use name")
}

func TestLoadSecretsWarnOnDeprecatedExternalName(t *testing.T) {
	buf, cleanup := patchLogrus()
	defer cleanup()

	project, err := loadYAML(`
name: test-warn-on-deprecated-external-name
secrets:
  foo:
     external:
       name: oops
`)
	assert.NilError(t, err)
	expected := types.Secrets{
		"foo": {
			Name:     "oops",
			External: types.External{External: true},
		},
	}
	assert.Check(t, is.DeepEqual(expected, project.Secrets))
	assert.Check(t, is.Contains(buf.String(), "secrets.foo: external.name is deprecated. Please set name and external: true"))
}

func TestLoadNetworksWarnOnDeprecatedExternalName(t *testing.T) {
	buf, cleanup := patchLogrus()
	defer cleanup()

	project, err := loadYAML(`
name: test-warn-on-deprecated-external-name
networks:
  foo:
    external:
      name: oops
`)
	assert.NilError(t, err)
	assert.NilError(t, err)
	expected := types.Networks{
		"foo": {
			Name:     "oops",
			External: types.External{External: true},
		},
	}
	assert.Check(t, is.DeepEqual(expected, project.Networks))
	assert.Check(t, is.Contains(buf.String(), "networks.foo: external.name is deprecated. Please set name and external: true"))

}

func TestLoadNetworkInvalidExternalNameAndNameCombination(t *testing.T) {
	_, err := loadYAML(`
name: load-network-invalid-external-name-and-name-combination
networks:
  foo:
    name: user_specified_name
    external:
      name: external_name
`)

	assert.ErrorContains(t, err, "networks.foo: name and external.name conflict; only use name")
}

func TestLoadNetworkWithName(t *testing.T) {
	config, err := loadYAML(`
name: load-network-with-name
services:
  hello-world:
    image: redis:alpine
    networks:
      - network1
      - network3

networks:
  network1:
    name: network2
  network3:
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	expected := &types.Project{
		Name:       "load-network-with-name",
		WorkingDir: workingDir,
		Services: types.Services{
			{
				Name:  "hello-world",
				Image: "redis:alpine",
				Scale: 1,
				Networks: map[string]*types.ServiceNetworkConfig{
					"network1": nil,
					"network3": nil,
				},
			},
		},
		Networks: map[string]types.NetworkConfig{
			"network1": {Name: "network2"},
			"network3": {},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-network-with-name",
		},
	}
	assert.DeepEqual(t, config, expected, cmpopts.EquateEmpty())
}

func TestLoadNetworkLinkLocalIPs(t *testing.T) {
	config, err := loadYAML(`
name: load-network-link-local-ips
services:
  foo:
    image: alpine
    networks:
      network1:
        ipv4_address: 10.1.0.100
        ipv6_address: 2001:db8:0:1::100
        link_local_ips:
          - fe80::1:95ff:fe20:100
networks:
  network1:
    driver: bridge
    enable_ipv6: true
    name: network1
    ipam:
      config:
        - subnet: 10.1.0.0/16
        - subnet: 2001:db8:0:1::/64
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	expected := &types.Project{
		Name:       "load-network-link-local-ips",
		WorkingDir: workingDir,
		Services: types.Services{
			{
				Name:  "foo",
				Image: "alpine",
				Scale: 1,
				Networks: map[string]*types.ServiceNetworkConfig{
					"network1": {
						Ipv4Address: "10.1.0.100",
						Ipv6Address: "2001:db8:0:1::100",
						LinkLocalIPs: []string{
							"fe80::1:95ff:fe20:100",
						},
					},
				},
			},
		},
		Networks: map[string]types.NetworkConfig{
			"network1": {
				Name:       "network1",
				Driver:     "bridge",
				EnableIPv6: true,
				Ipam: types.IPAMConfig{
					Config: []*types.IPAMPool{
						{Subnet: "10.1.0.0/16"},
						{Subnet: "2001:db8:0:1::/64"},
					},
				},
			},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-network-link-local-ips",
		},
	}
	assert.DeepEqual(t, config, expected, cmpopts.EquateEmpty())
}

func TestLoadInit(t *testing.T) {
	booleanTrue := true
	booleanFalse := false

	var testcases = []struct {
		doc  string
		yaml string
		init *bool
	}{
		{
			doc: "no init defined",
			yaml: `
name: no-init-defined
services:
  foo:
    image: alpine`,
		},
		{
			doc: "has true init",
			yaml: `
name: has-true-init
services:
  foo:
    image: alpine
    init: true`,
			init: &booleanTrue,
		},
		{
			doc: "has false init",
			yaml: `
name: has-false-init
services:
  foo:
    image: alpine
    init: false`,
			init: &booleanFalse,
		},
	}
	for _, testcase := range testcases {
		testcase := testcase
		t.Run(testcase.doc, func(t *testing.T) {
			config, err := loadYAML(testcase.yaml)
			assert.NilError(t, err)
			assert.Check(t, is.Len(config.Services, 1))
			assert.Check(t, is.DeepEqual(config.Services[0].Init, testcase.init))
		})
	}
}

func TestLoadSysctls(t *testing.T) {
	config, err := loadYAML(`
name: load-sysctls
services:
  web:
    image: busybox
    sysctls:
      - net.core.somaxconn=1024
      - net.ipv4.tcp_syncookies=0
      - testing.one.one=
      - testing.one.two
`)
	assert.NilError(t, err)

	expected := types.Mapping{
		"net.core.somaxconn":      "1024",
		"net.ipv4.tcp_syncookies": "0",
		"testing.one.one":         "",
		"testing.one.two":         "",
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].Sysctls))

	config, err = loadYAML(`
name: load-sysctls
services:
  web:
    image: busybox
    sysctls:
      net.core.somaxconn: 1024
      net.ipv4.tcp_syncookies: 0
      testing.one.one: ""
      testing.one.two:
`)
	assert.NilError(t, err)

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services[0].Sysctls))
}

func TestLoadTemplateDriver(t *testing.T) {
	config, err := loadYAML(`
name: load-template-driver
services:
  hello-world:
    image: redis:alpine
    secrets:
      - secret
    configs:
      - config

configs:
  config:
    name: config
    external: true
    template_driver: config-driver

secrets:
  secret:
    name: secret
    external: true
    template_driver: secret-driver
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	expected := &types.Project{
		Name:       "load-template-driver",
		WorkingDir: workingDir,
		Services: types.Services{
			{
				Name:  "hello-world",
				Image: "redis:alpine",
				Configs: []types.ServiceConfigObjConfig{
					{
						Source: "config",
					},
				},
				Scale: 1,
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "secret",
					},
				},
			},
		},
		Configs: map[string]types.ConfigObjConfig{
			"config": {
				Name:           "config",
				External:       types.External{External: true},
				TemplateDriver: "config-driver",
			},
		},
		Secrets: map[string]types.SecretConfig{
			"secret": {
				Name:           "secret",
				External:       types.External{External: true},
				TemplateDriver: "secret-driver",
			},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-template-driver",
		},
	}
	assert.DeepEqual(t, config, expected, cmpopts.EquateEmpty())
}

func TestLoadSecretDriver(t *testing.T) {
	config, err := loadYAML(`
name: load-secret-driver
services:
  hello-world:
    image: redis:alpine
    secrets:
      - secret
    configs:
      - config

configs:
  config:
    name: config
    external: true

secrets:
  secret:
    name: secret
    driver: secret-bucket
    driver_opts:
      OptionA: value for driver option A
      OptionB: value for driver option B
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)

	expected := &types.Project{
		Name:       "load-secret-driver",
		WorkingDir: workingDir,
		Services: types.Services{
			{
				Name:  "hello-world",
				Image: "redis:alpine",
				Configs: []types.ServiceConfigObjConfig{
					{
						Source: "config",
					},
				},
				Scale: 1,
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "secret",
					},
				},
			},
		},
		Configs: map[string]types.ConfigObjConfig{
			"config": {
				Name:     "config",
				External: types.External{External: true},
			},
		},
		Secrets: map[string]types.SecretConfig{
			"secret": {
				Name:   "secret",
				Driver: "secret-bucket",
				DriverOpts: map[string]string{
					"OptionA": "value for driver option A",
					"OptionB": "value for driver option B",
				},
			},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-secret-driver",
		},
	}
	assert.DeepEqual(t, config, expected, cmpopts.EquateEmpty())
}

func TestComposeFileWithVersion(t *testing.T) {
	b, err := os.ReadFile("testdata/compose-test-with-version.yaml")
	assert.NilError(t, err)

	homeDir, err := os.UserHomeDir()
	assert.NilError(t, err)
	env := map[string]string{"HOME": homeDir, "QUX": "qux_from_environment"}
	config, err := loadYAMLWithEnv(string(b), env)
	assert.NilError(t, err)

	expectedConfig := withVersionExampleConfig()

	sort.Slice(config.Services, func(i, j int) bool {
		return config.Services[i].Name > config.Services[j].Name
	})
	assert.Check(t, is.DeepEqual(expectedConfig.Services, config.Services))
	assert.Check(t, is.DeepEqual(expectedConfig.Networks, config.Networks))
	assert.Check(t, is.DeepEqual(expectedConfig.Volumes, config.Volumes))
}

func TestLoadWithExtends(t *testing.T) {
	b, err := os.ReadFile("testdata/compose-test-extends.yaml")
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: "testdata",
		ConfigFiles: []types.ConfigFile{
			{Filename: "testdata/compose-test-extends.yaml", Content: b},
		},
		Environment: map[string]string{},
	}

	actual, err := Load(configDetails)
	assert.NilError(t, err)

	extendsDir := filepath.Join("testdata", "subdir")

	expectedEnvFilePath := filepath.Join(extendsDir, "extra.env")

	expServices := types.Services{
		{
			Name:          "importer",
			Image:         "nginx",
			ContainerName: "imported",
			Environment: types.MappingWithEquals{
				"SOURCE": strPtr("extends"),
			},
			EnvFile:  []string{expectedEnvFilePath},
			Networks: map[string]*types.ServiceNetworkConfig{"default": nil},
			Volumes: []types.ServiceVolumeConfig{{
				Type:   "bind",
				Source: "/opt/data",
				Target: "/var/lib/mysql",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			}},
			Scale: 1,
		},
	}
	assert.Check(t, is.DeepEqual(expServices, actual.Services))
}

func TestLoadWithExtendsWithContextUrl(t *testing.T) {
	b, err := os.ReadFile("testdata/compose-test-extends-with-context-url.yaml")
	assert.NilError(t, err)

	configDetails := types.ConfigDetails{
		WorkingDir: "testdata",
		ConfigFiles: []types.ConfigFile{
			{Filename: "testdata/compose-test-extends-with-context-url.yaml", Content: b},
		},
		Environment: map[string]string{},
	}

	actual, err := Load(configDetails)
	assert.NilError(t, err)

	expServices := types.Services{
		{
			Name: "importer-with-https-url",
			Build: &types.BuildConfig{
				Context:    "https://github.com/docker/compose.git",
				Dockerfile: "Dockerfile",
			},
			Environment: types.MappingWithEquals{},
			Networks:    map[string]*types.ServiceNetworkConfig{"default": nil},
			Scale:       1,
		},
	}
	assert.Check(t, is.DeepEqual(expServices, actual.Services))
}

func TestServiceDeviceRequestCountIntegerType(t *testing.T) {
	_, err := loadYAML(`
name: service-device-request-count
services:
  hello-world:
    image: redis:alpine
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              capabilities: [gpu]
              count: 1
`)
	assert.NilError(t, err)
}

func TestServiceDeviceRequestCountStringType(t *testing.T) {
	_, err := loadYAML(`
name: service-device-request-count
services:
  hello-world:
    image: redis:alpine
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              capabilities: [gpu]
              count: all
`)
	assert.NilError(t, err)
}

func TestServiceDeviceRequestCountIntegerAsStringType(t *testing.T) {
	_, err := loadYAML(`
name: service-device-request-count-type
services:
  hello-world:
    image: redis:alpine
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              capabilities: [gpu]
              count: "1"
`)
	assert.NilError(t, err)
}

func TestServiceDeviceRequestCountInvalidStringType(t *testing.T) {
	_, err := loadYAML(`
name: service-device-request-count-type
services:
  hello-world:
    image: redis:alpine
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              capabilities: [gpu]
              count: some_string
`)
	assert.ErrorContains(t, err, `invalid value "some_string", the only value allowed is 'all' or a number`)
}

func TestServicePullPolicy(t *testing.T) {
	actual, err := loadYAML(`
name: service-pull-policy
services:
  hello-world:
    image: redis:alpine
    pull_policy: always
`)
	assert.NilError(t, err)
	svc, err := actual.GetService("hello-world")
	assert.NilError(t, err)
	assert.Equal(t, "always", svc.PullPolicy)
}

func TestEmptyList(t *testing.T) {
	_, err := loadYAML(`
name: empty-list
services:
  test:
    image: nginx:latest
    ports: []
`)
	assert.NilError(t, err)
}

func TestLoadServiceWithEnvFile(t *testing.T) {
	file, err := os.CreateTemp("", "test-compose-go")
	assert.NilError(t, err)
	defer os.Remove(file.Name())

	_, err = file.Write([]byte("HALLO=$TEST"))
	assert.NilError(t, err)

	p := &types.Project{
		Environment: map[string]string{
			"TEST": "YES",
		},
		Services: []types.ServiceConfig{
			{
				Name:    "Test",
				EnvFile: []string{file.Name()},
			},
		},
	}
	err = p.ResolveServicesEnvironment(false)
	assert.NilError(t, err)
	service, err := p.GetService("Test")
	assert.NilError(t, err)
	assert.Equal(t, "YES", *service.Environment["HALLO"])
}

func TestLoadNoSSHInBuildConfig(t *testing.T) {
	actual, err := loadYAML(`
name: load-no-ssh-in-build-config
services:
  test:
    build:
      context: .
`)
	assert.NilError(t, err)
	svc, err := actual.GetService("test")
	assert.NilError(t, err)
	assert.Check(t, svc.Build.SSH == nil)
}

func TestLoadSSHWithoutValueInBuildConfig(t *testing.T) {
	_, err := loadYAML(`
name: load-ssh-without-value-in-build-config
services:
  test:
    build:
      context: .
      ssh:
`)
	assert.ErrorContains(t, err, "services.test.build.ssh must be a mapping")
}

func TestLoadLegacyBoolean(t *testing.T) {
	actual, err := loadYAML(`
name: load-legacy-boolean
services:
  test:
    init: yes # used to be a valid YAML bool, removed in YAML 1.2
`)
	assert.NilError(t, err)
	assert.Check(t, *actual.Services[0].Init)
}

func TestLoadSSHWithDefaultValueInBuildConfig(t *testing.T) {
	actual, err := loadYAML(`
name: load-ssh-with-default-value-in-build-config
services:
  test:
    build:
      context: .
      ssh: [default]
`)
	assert.NilError(t, err)
	svc, err := actual.GetService("test")
	assert.NilError(t, err)
	sshValue, err := svc.Build.SSH.Get("default")
	assert.NilError(t, err)
	assert.Equal(t, "", sshValue)
}

func TestLoadSSHWithKeyValueInBuildConfig(t *testing.T) {
	actual, err := loadYAML(`
name: load-ssh-with-key-value-in-build-config
services:
  test:
    build:
      context: .
      ssh:
        key1: value1
`)
	assert.NilError(t, err)
	svc, err := actual.GetService("test")
	assert.NilError(t, err)
	sshValue, err := svc.Build.SSH.Get("key1")
	assert.NilError(t, err)
	assert.Equal(t, "value1", sshValue)
}

func TestLoadSSHWithKeysValuesInBuildConfig(t *testing.T) {
	actual, err := loadYAML(`
name: load-ssh-with-keys-values-in-build-config
services:
  test:
    build:
      context: .
      ssh:
        - key1=value1
        - key2=value2
`)
	assert.NilError(t, err)
	svc, err := actual.GetService("test")
	assert.NilError(t, err)

	sshValue, err := svc.Build.SSH.Get("key1")
	assert.NilError(t, err)
	assert.Equal(t, "value1", sshValue)

	sshValue, err = svc.Build.SSH.Get("key2")
	assert.NilError(t, err)
	assert.Equal(t, "value2", sshValue)
}

func TestProjectNameInterpolation(t *testing.T) {
	t.Run("project name simple interpolation", func(t *testing.T) {
		yaml := `
name: interpolated
services:
  web:
    image: web
    container_name: ${COMPOSE_PROJECT_NAME}-web
`
		configDetails := buildConfigDetails(yaml, map[string]string{})

		actual, err := Load(configDetails)
		assert.NilError(t, err)
		svc, err := actual.GetService("web")
		assert.NilError(t, err)
		assert.Equal(t, "interpolated-web", svc.ContainerName)
	})

	t.Run("project name interpolation with override", func(t *testing.T) {
		yaml1 := `
name: interpolated
services:
  web:
    image: web
    container_name: ${COMPOSE_PROJECT_NAME}-web
`
		yaml2 := `
name: overrided
services:
  db:
    image: db
    container_name: ${COMPOSE_PROJECT_NAME}-db
`
		yaml3 := `
services:
  proxy:
    image: proxy
    container_name: ${COMPOSE_PROJECT_NAME}-proxy
`
		configDetails := buildConfigDetailsMultipleFiles(map[string]string{}, yaml1, yaml2, yaml3)

		actual, err := Load(configDetails)
		assert.NilError(t, err)
		svc, err := actual.GetService("web")
		assert.NilError(t, err)
		assert.Equal(t, "overrided-web", svc.ContainerName)

		svc, err = actual.GetService("db")
		assert.NilError(t, err)
		assert.Equal(t, "overrided-db", svc.ContainerName)

		svc, err = actual.GetService("proxy")
		assert.NilError(t, err)
		assert.Equal(t, "overrided-proxy", svc.ContainerName)
	})

	t.Run("project name env variable interpolation", func(t *testing.T) {
		yaml := `
name: interpolated
services:
  web:
    image: web
    container_name: ${COMPOSE_PROJECT_NAME}-web
`
		configDetails := buildConfigDetails(yaml, map[string]string{"COMPOSE_PROJECT_NAME": "env-var"})
		actual, err := Load(configDetails)
		assert.NilError(t, err)
		svc, err := actual.GetService("web")
		assert.NilError(t, err)
		assert.Equal(t, "env-var-web", svc.ContainerName)
	})
}

func TestLoadWithBindMountVolume(t *testing.T) {
	dict := `
name: load-with-bind-mount-volume
services:
  web:
    image: web
    volumes:
     - data:/data
volumes:
  data:
    driver: local
    driver_opts:
      type: 'none'
      o: 'bind'
      device: './data'
`
	configDetails := buildConfigDetails(dict, nil)

	project, err := Load(configDetails)
	assert.NilError(t, err)
	path := project.Volumes["data"].DriverOpts["device"]
	assert.Check(t, filepath.IsAbs(path))
}

func TestLoadServiceExtension(t *testing.T) {
	dict := `
name: test
services:
  extension: # this name should be allowed
    image: web
    x-foo: bar
`
	configDetails := buildConfigDetails(dict, nil)

	project, err := Load(configDetails)
	assert.NilError(t, err)
	assert.Equal(t, project.Services[0].Name, "extension")
	assert.Equal(t, project.Services[0].Extensions["x-foo"], "bar")
}

func TestDeviceWriteBps(t *testing.T) {
	p, err := loadYAML(`
        name: test
        services:
          foo:
            image: busybox
            blkio_config:
              device_read_bps:
              - path: /dev/test
                rate: 1024k
              device_write_bps:
              - path: /dev/test
                rate: 1024
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services, types.Services{
		{
			Name:        "foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
			Scale:       1,
			BlkioConfig: &types.BlkioConfig{
				DeviceReadBps: []types.ThrottleDevice{
					{
						Path: "/dev/test",
						Rate: types.UnitBytes(1024 * 1024),
					},
				},
				DeviceWriteBps: []types.ThrottleDevice{
					{
						Path: "/dev/test",
						Rate: types.UnitBytes(1024),
					},
				},
			},
		},
	})

}

func TestInvalidProjectNameType(t *testing.T) {
	p, err := loadYAML(`name: 123`)
	assert.Error(t, err, "validating filename0.yml: name must be a string")
	assert.Assert(t, is.Nil(p))
}

func TestNumericIDs(t *testing.T) {
	p, err := loadYAML(`
name: 'test-numeric-ids'
services:
  foo:
    image: busybox
    volumes:
      - 0:/foo

volumes:
  '0': {}
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services, types.Services{
		{
			Name:        "foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
			Scale:       1,
			Volumes: []types.ServiceVolumeConfig{
				{
					Type:   types.VolumeTypeVolume,
					Source: "0",
					Target: "/foo",
					Volume: &types.ServiceVolumeVolume{},
				},
			},
		},
	})
}

func TestXService(t *testing.T) {
	p, err := loadYAML(`
name: 'test-x-service'
services:
  x-foo:
    image: busybox
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services, types.Services{
		{
			Name:        "x-foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
			Scale:       1,
		},
	})
}

func TestLoadWithInclude(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	p, err := Load(buildConfigDetails(`
name: 'test-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env

services:
  foo:
    image: busybox
    depends_on:
      - imported
`, nil), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services, types.Services{
		{
			Name:        "foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
			Scale:       1,
			DependsOn:   types.DependsOnConfig{"imported": {Condition: "service_started", Required: true}},
		},
		{
			Name:          "imported",
			ContainerName: "extends", // as defined by ./testdata/subdir/extra.env
			Environment:   types.MappingWithEquals{"SOURCE": strPtr("extends")},
			EnvFile: types.StringList{
				filepath.Join(workingDir, "testdata", "subdir", "extra.env"),
			},
			Image: "nginx",
			Scale: 1,
			Volumes: []types.ServiceVolumeConfig{
				{
					Type:   "bind",
					Source: "/opt/data",
					Target: "/var/lib/mysql",
					Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
				},
			},
		},
	})
	assert.DeepEqual(t, p.IncludeReferences, map[string][]types.IncludeConfig{
		filepath.Join(workingDir, "filename0.yml"): {
			{
				Path:             []string{filepath.Join(workingDir, "testdata", "subdir", "compose-test-extends-imported.yaml")},
				ProjectDirectory: workingDir,
				EnvFile:          []string{filepath.Join(workingDir, "testdata", "subdir", "extra.env")},
			},
		},
	})
	assert.NilError(t, err)

	p, err = Load(buildConfigDetails(`
name: 'test-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env

services:
  foo:
    image: busybox
    depends_on:
      - imported
`, map[string]string{"SOURCE": "override"}), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services[1].ContainerName, "override")
}

func TestLoadWithIncludeCycle(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	_, err = Load(types.ConfigDetails{
		WorkingDir: filepath.Join(workingDir, "testdata"),
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join(workingDir, "testdata", "compose-include-cycle.yaml"),
			},
		},
	})
	assert.Check(t, strings.HasPrefix(err.Error(), "include cycle detected"))
}

func TestLoadWithMultipleInclude(t *testing.T) {
	// include same service twice should not trigger an error
	p, err := Load(buildConfigDetails(`
name: 'test-multi-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env
  - path: ./testdata/compose-include.yaml

services:
  foo:
    image: busybox
    depends_on:
      - imported
`, map[string]string{"SOURCE": "override"}), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services[1].ContainerName, "override")

	// include 2 different services with same name should trigger an error
	p, err = Load(buildConfigDetails(`
name: 'test-multi-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env
  - path: ./testdata/compose-include.yaml
    env_file: ./testdata/subdir/extra.env


services:
  bar:
    image: busybox
`, map[string]string{"SOURCE": "override"}), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.ErrorContains(t, err, "defines conflicting service bar", err)
}

func TestLoadWithDependsOn(t *testing.T) {
	p, err := loadYAML(`
name: test-depends-on
services:
  foo:
    image: nginx
    depends_on:
      bar:
        condition: service_started
      baz:
        condition: service_healthy
        required: false
      qux:
        condition: service_completed_successfully
        required: true
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services, types.Services{
		{
			Name:        "foo",
			Image:       "nginx",
			Environment: types.MappingWithEquals{},
			Scale:       1,
			DependsOn: types.DependsOnConfig{
				"bar": {Condition: types.ServiceConditionStarted, Required: true},
				"baz": {Condition: types.ServiceConditionHealthy, Required: false},
				"qux": {Condition: types.ServiceConditionCompletedSuccessfully, Required: true},
			},
		},
	})
}

type customLoader struct {
	prefix string
}

func (c customLoader) Accept(s string) bool {
	return strings.HasPrefix(s, c.prefix+":")
}

func (c customLoader) Load(ctx context.Context, s string) (string, error) {
	path := filepath.Join("testdata", c.prefix, s[len(c.prefix)+1:])
	_, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func TestLoadWithRemoteResources(t *testing.T) {
	config := buildConfigDetails(`
name: test-remote-resources
services:
  foo:
    extends:
      file: remote:compose.yaml
      service: foo

`, nil)
	p, err := LoadWithContext(context.Background(), config, func(options *Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.ResourceLoaders = []ResourceLoader{
			customLoader{prefix: "remote"},
		}
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services, types.Services{
		{
			Name:        "foo",
			Image:       "foo",
			Environment: types.MappingWithEquals{"FOO": strPtr("BAR")},
			EnvFile: types.StringList{
				filepath.Join(config.WorkingDir, "testdata", "remote", "env"),
			},
			Scale: 1,
			Volumes: []types.ServiceVolumeConfig{
				{
					Type:   types.VolumeTypeBind,
					Source: filepath.Join(config.WorkingDir, "testdata", "remote"),
					Target: "/foo",
					Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
				},
			},
		},
	})
}

func TestLoadWithMissingResources(t *testing.T) {
	config := buildConfigDetails(`
name: test-missing-resources
services:
  foo:
    extends:
      file: remote:unavailable.yaml
      service: foo

`, nil)
	_, err := LoadWithContext(context.Background(), config, func(options *Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.ResourceLoaders = []ResourceLoader{
			customLoader{prefix: "remote"},
		}
	})
	assert.Check(t, os.IsNotExist(err))
}

func TestLoadWithNestedResources(t *testing.T) {
	config := buildConfigDetails(`
name: test-nested-resources
include:
  - remote:nested/compose.yaml
`, nil)
	_, err := LoadWithContext(context.Background(), config, func(options *Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.ResourceLoaders = []ResourceLoader{
			customLoader{prefix: "remote"},
		}
	})
	assert.NilError(t, err)
}

func TestLoadWithResourcesCycle(t *testing.T) {
	config := buildConfigDetails(`
name: test-resources-cycle
services:
  foo:
    extends:
      file: remote:cycle/compose.yaml
      service: foo

`, nil)
	_, err := LoadWithContext(context.Background(), config, func(options *Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.ResourceLoaders = []ResourceLoader{
			customLoader{prefix: "remote"},
		}
	})
	assert.ErrorContains(t, err, "Circular reference")
}

func TestLoadMulmtiDocumentYaml(t *testing.T) {
	project, err := loadYAML(`
name: load-multi-docs
services:
  test:
    image: nginx:latest
---
services:
  test:
    image: nginx:override

`)
	assert.NilError(t, err)
	assert.Equal(t, project.Services[0].Image, "nginx:override")
}

func TestLoadDevelopConfig(t *testing.T) {
	project, err := Load(buildConfigDetails(`
name: load-develop
services:
  frontend:
    image: example/webapp
    build: ./webapp
    develop:
      watch:
        # sync static content
        - path: ./webapp/html
          action: sync
          target: /var/www
          ignore:
            - node_modules/

  backend:
    image: example/backend
    build: ./backend
    develop:
      watch:
        # rebuild image and recreate service
        - path: ./backend/src
          action: rebuild
  proxy:
    image: example/proxy
    build: ./proxy
    develop:
      watch: 
        # rebuild image and recreate service
        - path: ./proxy/proxy.conf
          action: sync+restart
`, nil), func(options *Options) {
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	frontend, err := project.GetService("frontend")
	assert.NilError(t, err)
	assert.DeepEqual(t, *frontend.Develop, types.DevelopConfig{
		Watch: []types.Trigger{
			{
				Path:   "./webapp/html",
				Action: types.WatchActionSync,
				Target: "/var/www",
				Ignore: []string{"node_modules/"},
			},
		},
	})
	backend, err := project.GetService("backend")
	assert.NilError(t, err)
	assert.DeepEqual(t, *backend.Develop, types.DevelopConfig{
		Watch: []types.Trigger{
			{
				Path:   "./backend/src",
				Action: types.WatchActionRebuild,
			},
		},
	})
	proxy, err := project.GetService("proxy")
	assert.NilError(t, err)
	assert.DeepEqual(t, *proxy.Develop, types.DevelopConfig{
		Watch: []types.Trigger{
			{
				Path:   "./proxy/proxy.conf",
				Action: types.WatchActionSyncRestart,
			},
		},
	})
}

func TestBadServiceConfig(t *testing.T) {
	yaml := `name: scratch
services:
  redis:
    image: redis:6.2.6-alpine
    network_mode: bridge
    networks:
      gratheon: null
networks:
  gratheon:
    name: scratch_gratheon
`
	_, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	})
	assert.ErrorContains(t, err, "service redis declares mutually exclusive `network_mode` and `networks`")
}
