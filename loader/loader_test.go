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
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"

	"github.com/compose-spec/compose-go/v2/types"
	"github.com/compose-spec/compose-go/v2/utils"
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
			Content:  []byte(yaml),
		})
	}
	return configFiles
}

func loadYAML(yaml string) (*types.Project, error) {
	return loadYAMLWithEnv(yaml, nil)
}

func loadYAMLWithEnv(yaml string, env map[string]string) (*types.Project, error) {
	return LoadWithContext(context.TODO(), buildConfigDetails(yaml, env), func(options *Options) {
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
		Mode:      "ingress",
		Target:    53,
		Published: "10053",
		Protocol:  "udp",
	},
	{
		Mode:      "host",
		Target:    22,
		Published: "10022",
		Protocol:  "tcp",
	},
}

func strPtr(val string) *string {
	return &val
}

var sampleConfig = types.Config{
	Services: types.Services{
		"foo": {
			Name:        "foo",
			Image:       "busybox",
			Environment: map[string]*string{},
			Networks: map[string]*types.ServiceNetworkConfig{
				"with_me": nil,
			},
		},
		"bar": {
			Name:        "bar",
			Image:       "busybox",
			Environment: map[string]*string{"FOO": strPtr("1")},
			Networks: map[string]*types.ServiceNetworkConfig{
				"with_ipam": nil,
			},
		},
	},
	Networks: types.Networks{
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
	Volumes: types.Volumes{
		"hello": {
			Driver: "default",
			DriverOpts: map[string]string{
				"beep": "boop",
			},
		},
	},
}

func TestLoad(t *testing.T) {
	actual, err := LoadWithContext(context.TODO(), buildConfigDetails(sampleYAML, nil), func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(sampleConfig.Services, actual.Services))
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
	actual, err := LoadWithContext(context.TODO(), types.ConfigDetails{
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
	assert.Check(t, is.DeepEqual(sampleConfig.Services, actual.Services))
	assert.Check(t, is.DeepEqual(sampleConfig.Networks, actual.Networks))
	assert.Check(t, is.DeepEqual(sampleConfig.Volumes, actual.Volumes))
}

func TestLoadExtensions(t *testing.T) {
	actual, err := loadYAML(`
name: load-extensions
x-project: project

configs:
  x-config-name:
    external: true
    x-config-ext: config
services:
  another:
    image: busybox
    depends_on:
      foo:
        condition: service_started
        x-depends-on: depends
  foo:
    image: busybox
    x-foo: bar
    healthcheck:
      x-healthcheck: health
    develop:
      x-dev: dev`)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(types.Extensions{
		"x-project": "project",
	}, actual.Extensions))
	assert.Check(t, is.Len(actual.Services, 2))
	service := actual.Services["foo"]
	assert.Check(t, is.Equal("busybox", service.Image))

	assert.Check(t, is.DeepEqual(types.Extensions{
		"x-foo": "bar",
	}, service.Extensions))
	assert.Check(t, is.DeepEqual(types.Extensions{
		"x-config-ext": "config",
	}, actual.Configs["x-config-name"].Extensions))
	assert.Check(t, is.DeepEqual(types.Extensions{
		"x-dev": "dev",
	}, service.Develop.Extensions))
	assert.Check(t, is.DeepEqual(types.Extensions{
		"x-healthcheck": "health",
	}, service.HealthCheck.Extensions))

	another := actual.Services["another"]
	assert.Check(t, is.DeepEqual(types.Extensions{
		"x-depends-on": "depends",
	}, another.DependsOn["foo"].Extensions))
}

func TestLoadExtends(t *testing.T) {
	actual, err := loadYAML(`
name: load-extends
services:
  foo:
    image: busybox
    extends: bar
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

	actual, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(tmpdir, "compose.yaml"),
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
		options.SetProjectName("project", true)
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(actual.Services, 2))

	svcA, err := actual.GetService("a")
	assert.NilError(t, err)
	assert.Equal(t, svcA.Build.Context, aDir)

	svcB, err := actual.GetService("b")
	assert.NilError(t, err)
	assert.Equal(t, svcB.Build.Context, tmpdir)
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
	assert.Check(t, is.Equal(actual.Services["foo"].CredentialSpec.Config, "0bt9dmxjvjiqermk6xrop3ekq"))
}

func TestParseAndLoad(t *testing.T) {
	actual, err := loadYAML(sampleYAML)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(sampleConfig.Services, actual.Services))
	assert.Check(t, is.DeepEqual(sampleConfig.Networks, actual.Networks))
	assert.Check(t, is.DeepEqual(sampleConfig.Volumes, actual.Volumes))
}

func TestInvalidTopLevelObjectType(t *testing.T) {
	_, err := loadYAML("1")
	assert.ErrorContains(t, err, "top-level object must be a mapping")

	_, err = loadYAML("\"hello\"")
	assert.ErrorContains(t, err, "top-level object must be a mapping")

	_, err = loadYAML("[\"hello\"]")
	assert.ErrorContains(t, err, "top-level object must be a mapping")
}

func TestNonStringKeys(t *testing.T) {
	_, err := loadYAML(`
123:
  foo:
    image: busybox
`)
	assert.ErrorContains(t, err, "non-string key at top level: 123")

	_, err = loadYAML(`
services:
  foo:
    image: busybox
  123:
    image: busybox
`)
	assert.ErrorContains(t, err, "non-string key in services: 123")

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
	assert.ErrorContains(t, err, "non-string key in networks.default.ipam.config[0]: 123")

	_, err = loadYAML(`
services:
  dict-env:
    image: busybox
    environment:
      1: FOO
`)
	assert.ErrorContains(t, err, "non-string key in services.dict-env.environment: 1")
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
		"MEU": strPtr("Shadoks"),
		"ZO":  nil,
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
	assert.ErrorContains(t, err, "services.dict-env.environment.FOO must be a boolean, null, number or string")
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

	assert.Check(t, is.DeepEqual(expectedLabels, config.Services["test"].Labels))
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
        mode: "$theint"
    secrets:
      - source: super
        mode: "$theint"
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

	config, err := LoadWithContext(context.TODO(), buildConfigDetails(dict, env), func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	duration, err := time.ParseDuration("60s")
	assert.NilError(t, err)
	typesDuration := types.Duration(duration)
	expected := &types.Project{
		Name:             "load-with-interpolation-cast-full",
		Environment:      env,
		WorkingDir:       workingDir,
		DisabledServices: types.Services{},
		Services: types.Services{
			"web": {
				Name: "web",
				Configs: []types.ServiceConfigObjConfig{
					{
						Source: "appconfig",
						Mode:   ptr(types.FileMode(0o555)),
					},
				},
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "super",
						Target: "/run/secrets/super",
						Mode:   ptr(types.FileMode(0o555)),
					},
				},
				HealthCheck: &types.HealthCheckConfig{
					Retries: ptr(uint64(555)),
					Disable: true,
				},
				Deploy: &types.DeployConfig{
					Replicas: ptr(555),
					UpdateConfig: &types.UpdateConfig{
						Parallelism:     ptr(uint64(555)),
						MaxFailureRatio: 3.14,
					},
					RollbackConfig: &types.UpdateConfig{
						Parallelism:     ptr(uint64(555)),
						MaxFailureRatio: 3.14,
					},
					RestartPolicy: &types.RestartPolicy{
						MaxAttempts: ptr(uint64(555)),
					},
					Placement: types.Placement{
						MaxReplicas: 555,
					},
				},
				Ports: []types.ServicePortConfig{
					{Target: 555, Mode: "ingress", Protocol: "tcp"},
					{Target: 34567, Mode: "ingress", Protocol: "tcp"},
					{Target: 555, Mode: "ingress", Protocol: "tcp", Published: "555", Extensions: map[string]interface{}{"x-foo-bar": true}},
				},
				Ulimits: map[string]*types.UlimitsConfig{
					"nproc":  {Single: 555},
					"nofile": {Hard: 555, Soft: 555},
				},
				Privileged:      true,
				ReadOnly:        true,
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
			"appconfig": {External: true},
		},
		Secrets: map[string]types.SecretConfig{
			"super": {External: true},
		},
		Volumes: map[string]types.VolumeConfig{
			"data": {External: true},
		},
		Networks: map[string]types.NetworkConfig{
			"back": {},
			"front": {
				External:   true,
				Internal:   true,
				Attachable: true,
			},
		},
	}

	assertEqual(t, expected, config)
}

func TestLoadWithLabelFile(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	config, err := loadYAML(`
name: load-label-file
services:
  service_1:
    label_file: ./example1.label
  service_2:
    label_file:
      - ./example2.label
`)
	assert.NilError(t, err)

	expected := &types.Project{
		Name:        "load-label-file",
		Environment: types.Mapping{"COMPOSE_PROJECT_NAME": "load-label-file"},
		WorkingDir:  workingDir,
		Services: types.Services{
			"service_1": {
				Name:        "service_1",
				Environment: types.MappingWithEquals{},
				Labels: types.Labels{
					"BAR":                   "bar_from_label_file",
					"BAZ":                   "baz_from_label_file",
					"FOO":                   "foo_from_label_file",
					"LABEL.WITH.DOT":        "ok",
					"LABEL_WITH_UNDERSCORE": "ok",
				},
				LabelFiles: []string{
					filepath.Join(workingDir, "example1.label"),
				},
			},
			"service_2": {
				Name:        "service_2",
				Environment: types.MappingWithEquals{},
				Labels: types.Labels{
					"BAR": "bar_from_label_file_2",
					"QUX": "quz_from_label_file_2",
				},
				LabelFiles: []string{
					filepath.Join(workingDir, "example2.label"),
				},
			},
		},
	}
	assertEqual(t, config, expected)
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

	_, err := LoadWithContext(context.TODO(), configDetails)
	assert.NilError(t, err)
}

func TestDiscardEnvFileOption(t *testing.T) {
	dict := `name: test
services:
  web:
    image: nginx
    env_file:
     - example1.env
     - path: example2.env
       required: false
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
	configWithEnvFiles, err := LoadWithContext(context.TODO(), configDetails, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, configWithEnvFiles.Services["web"].EnvFiles, []types.EnvFile{
		{
			Path:     "example1.env",
			Required: true,
		},
		{
			Path:     "example2.env",
			Required: false,
		},
	})
	assert.DeepEqual(t, configWithEnvFiles.Services["web"].Environment, expectedEnvironmentMap)

	// Custom behavior removes the `env_file` entries
	configWithoutEnvFiles, err := LoadWithContext(context.TODO(), configDetails, WithDiscardEnvFiles)
	assert.NilError(t, err)
	assert.Equal(t, len(configWithoutEnvFiles.Services["web"].EnvFiles), 0)
	assert.DeepEqual(t, configWithoutEnvFiles.Services["web"].Environment, expectedEnvironmentMap)
}

func TestDecodeErrors(t *testing.T) {
	dict := "name: test\nservices:\n  web:\n    image: nginx\n\tbuild: ."

	configDetails := buildConfigDetails(dict, nil)
	_, err := LoadWithContext(context.TODO(), configDetails)
	assert.Error(t, err, "yaml: line 4: found a tab character that violates indentation")
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
     shm_size: 2gb
`
	configDetails := buildConfigDetails(dict, nil)
	actual, err := LoadWithContext(context.TODO(), configDetails)
	assert.NilError(t, err)

	wd, _ := os.Getwd()
	assert.DeepEqual(t, actual.Services["web"].Build, &types.BuildConfig{
		Context:    wd,
		Dockerfile: "Dockerfile",
	})

	assert.DeepEqual(t, actual.Services["db"].Build, &types.BuildConfig{
		Context:    filepath.Join(wd, "db"),
		Dockerfile: "Dockerfile",
		ShmSize:    types.UnitBytes(2 * 1024 * 1024 * 1024),
	})
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

	_, err := LoadWithContext(context.TODO(), configDetails)
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
	assert.ErrorContains(t, err, "additional properties 'impossible' not allowed")
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
	foo := project.Services["foo"]
	assert.Equal(t, *foo.Scale, 2)
}

func ptr[T any](t T) *T {
	return &t
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
	assert.Check(t, is.DeepEqual(expectedConfig.Services, config.Services))
	assert.Check(t, is.DeepEqual(expectedConfig.Networks, config.Networks))
	assert.Check(t, is.DeepEqual(expectedConfig.Volumes, config.Volumes))
	assert.Check(t, is.DeepEqual(expectedConfig.Secrets, config.Secrets, cmpopts.IgnoreUnexported(types.SecretConfig{})))
	assert.Check(t, is.DeepEqual(expectedConfig.Configs, config.Configs, cmpopts.IgnoreUnexported(types.ConfigObjConfig{})))
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
		assert.Check(t, is.Len(config.Services["tmpfs"].Volumes, 1))
		assert.Check(t, is.DeepEqual(expected, config.Services["tmpfs"].Volumes[0]))
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
	assert.ErrorContains(t, err, "services.tmpfs.volumes.0 additional properties 'foo' not allowed")
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
  test:
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
  test:
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
  test:
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
  test:
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
			assert.Check(t, is.Len(config.Services["test"].Volumes, 1))
			assert.Check(t, is.DeepEqual(tc.expected, config.Services["test"].Volumes[0]))
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
	assert.Check(t, is.Len(config.Services["bind"].Volumes, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services["bind"].Volumes[0]))
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
	assert.Check(t, is.Len(config.Services["tmpfs"].Volumes, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services["tmpfs"].Volumes[0]))
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
	assert.ErrorContains(t, err, "services.tmpfs.volumes.0.tmpfs.size must be greater than or equal to 0")
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

func TestLoadVolumeSubpath(t *testing.T) {
	config, err := loadYAML(`
name: load-volume-subpath
services:
  test:
    image: alpine:latest
    volumes:
      - type: volume
        source: asdf
        target: /app
        volume:
          subpath: etc/settings
`)
	assert.Check(t, err)
	assert.Check(t, is.Equal(config.Services["test"].Volumes[0].Volume.Subpath, "etc/settings"))
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
	assert.Check(t, is.DeepEqual(samplePortsConfig, config.Services["web"].Ports))
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
	assert.Check(t, is.Len(config.Services["web"].Volumes, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services["web"].Volumes[0]))
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
		"alpha": []string{"50.31.209.229"},
		"zulu":  []string{"162.242.195.82"},
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services["web"].ExtraHosts))
}

func TestLoadExtraHostsList(t *testing.T) {
	config, err := loadYAML(`
name: load-extra-hosts-list
services:
  web:
    image: busybox
    extra_hosts:
      - "alpha:50.31.209.229"
      - "zulu:127.0.0.2"
      - "zulu:ff02::1"
`)
	assert.NilError(t, err)

	expected := types.HostsList{
		"alpha": []string{"50.31.209.229"},
		"zulu":  []string{"127.0.0.2", "ff02::1"},
	}

	assert.Assert(t, is.Len(config.Services, 1))
	assert.Check(t, is.DeepEqual(expected, config.Services["web"].ExtraHosts))
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
			External: true,
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
	assert.Check(t, is.Equal("invalid", actual.Services["foo"].Isolation))
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
			External: true,
		},
	}
	assert.Check(t, is.DeepEqual(expected, project.Secrets, cmpopts.IgnoreUnexported(types.SecretConfig{})))
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
			External: true,
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
			"hello-world": {
				Name:  "hello-world",
				Image: "redis:alpine",
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

func TestLoadIPv6Only(t *testing.T) {
	config, err := loadYAML(`
name: load-network-ipv6only
services:
  foo:
    image: alpine
    networks:
      network1:
networks:
  network1:
    driver: bridge
    enable_ipv4: false
    enable_ipv6: true
    name: network1
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	enableIPv4 := false
	enableIPv6 := true
	expected := &types.Project{
		Name:       "load-network-ipv6only",
		WorkingDir: workingDir,
		Services: types.Services{
			"foo": {
				Name:  "foo",
				Image: "alpine",
				Networks: map[string]*types.ServiceNetworkConfig{
					"network1": nil,
				},
			},
		},
		Networks: map[string]types.NetworkConfig{
			"network1": {
				Name:       "network1",
				Driver:     "bridge",
				EnableIPv4: &enableIPv4,
				EnableIPv6: &enableIPv6,
			},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-network-ipv6only",
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
	enableIPv6 := true
	expected := &types.Project{
		Name:       "load-network-link-local-ips",
		WorkingDir: workingDir,
		Services: types.Services{
			"foo": {
				Name:  "foo",
				Image: "alpine",
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
				EnableIPv6: &enableIPv6,
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

func TestLoadServiceNetworkDriverOpts(t *testing.T) {
	config, err := loadYAML(`
name: load-service-network-driver-opts
services:
  foo:
    image: alpine
    networks:
      network1:
        driver_opts:
          com.docker.network.endpoint.sysctls: "ipv6.conf.accept_ra=0"
networks:
  network1:
`)
	assert.NilError(t, err)

	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	expected := &types.Project{
		Name:       "load-service-network-driver-opts",
		WorkingDir: workingDir,
		Services: types.Services{
			"foo": {
				Name:  "foo",
				Image: "alpine",
				Networks: map[string]*types.ServiceNetworkConfig{
					"network1": {
						DriverOpts: types.Options{
							"com.docker.network.endpoint.sysctls": "ipv6.conf.accept_ra=0",
						},
					},
				},
			},
		},
		Networks: map[string]types.NetworkConfig{
			"network1": {},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-service-network-driver-opts",
		},
	}
	assert.DeepEqual(t, config, expected, cmpopts.EquateEmpty())
}

func TestLoadInit(t *testing.T) {
	booleanTrue := true
	booleanFalse := false

	testcases := []struct {
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
		t.Run(testcase.doc, func(t *testing.T) {
			config, err := loadYAML(testcase.yaml)
			assert.NilError(t, err)
			assert.Check(t, is.Len(config.Services, 1))
			assert.Check(t, is.DeepEqual(config.Services["foo"].Init, testcase.init))
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
	assert.Check(t, is.DeepEqual(expected, config.Services["web"].Sysctls))

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
	assert.Check(t, is.DeepEqual(expected, config.Services["web"].Sysctls))
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
			"hello-world": {
				Name:  "hello-world",
				Image: "redis:alpine",
				Configs: []types.ServiceConfigObjConfig{
					{
						Source: "config",
					},
				},
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "secret",
						Target: "/run/secrets/secret",
					},
				},
			},
		},
		Configs: map[string]types.ConfigObjConfig{
			"config": {
				Name:           "config",
				External:       true,
				TemplateDriver: "config-driver",
			},
		},
		Secrets: map[string]types.SecretConfig{
			"secret": {
				Name:           "secret",
				External:       true,
				TemplateDriver: "secret-driver",
			},
		},
		Environment: types.Mapping{
			"COMPOSE_PROJECT_NAME": "load-template-driver",
		},
	}
	assertEqual(t, expected, config)
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
			"hello-world": {
				Name:  "hello-world",
				Image: "redis:alpine",
				Configs: []types.ServiceConfigObjConfig{
					{
						Source: "config",
					},
				},
				Secrets: []types.ServiceSecretConfig{
					{
						Source: "secret",
						Target: "/run/secrets/secret",
					},
				},
			},
		},
		Configs: map[string]types.ConfigObjConfig{
			"config": {
				Name:     "config",
				External: true,
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
	assertEqual(t, config, expected)
}

func assertEqual(t *testing.T, config *types.Project, expected *types.Project) {
	assert.DeepEqual(t, config, expected, cmpopts.EquateEmpty(), cmpopts.IgnoreUnexported(types.SecretConfig{}), cmpopts.IgnoreUnexported(types.ConfigObjConfig{}))
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
			{Filename: "compose-test-extends.yaml", Content: b},
		},
		Environment: map[string]string{},
	}

	actual, err := LoadWithContext(context.TODO(), configDetails)
	assert.NilError(t, err)

	extendsDir := filepath.Join("testdata", "subdir")

	expectedEnvFilePath := filepath.Join(extendsDir, "extra.env")

	expServices := types.Services{
		"importer": {
			Name:          "importer",
			Image:         "nginx",
			ContainerName: "imported",
			Environment: types.MappingWithEquals{
				"SOURCE": strPtr("extends"),
			},
			EnvFiles: []types.EnvFile{
				{
					Path:     expectedEnvFilePath,
					Required: true,
				},
			},
			Networks: map[string]*types.ServiceNetworkConfig{"default": nil},
			Volumes: []types.ServiceVolumeConfig{{
				Type:   "bind",
				Source: "/opt/data",
				Target: "/var/lib/mysql",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			}},
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

	actual, err := LoadWithContext(context.TODO(), configDetails)
	assert.NilError(t, err)

	expServices := types.Services{
		"importer-with-https-url": {
			Name: "importer-with-https-url",
			Build: &types.BuildConfig{
				Context:    "https://github.com/docker/compose.git",
				Dockerfile: "Dockerfile",
			},
			Environment: types.MappingWithEquals{},
			Networks:    map[string]*types.ServiceNetworkConfig{"default": nil},
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
	project, err := loadYAML(`
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
	assert.Equal(t, project.Services["hello-world"].Deploy.Resources.Reservations.Devices[0].Count, types.DeviceCount(-1), err)
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

func TestServiceDeviceRequestWithoutCountAndDeviceIdsType(t *testing.T) {
	project, err := loadYAML(`
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
`)
	assert.NilError(t, err)
	assert.Equal(t, project.Services["hello-world"].Deploy.Resources.Reservations.Devices[0].Count, types.DeviceCount(-1), err)
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

func TestServiceDeviceRequestCountAndDeviceIdsExclusive(t *testing.T) {
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
              count: 2
              device_ids: ["my-device-id"]
`)
	assert.ErrorContains(t, err, `"count" and "device_ids" attributes are exclusive`)
}

func TestServiceDeviceRequestCapabilitiesMandatory(t *testing.T) {
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
              count: 2
`)
	assert.ErrorContains(t, err, "missing property 'capabilities'")
}

func TestServiceGpus(t *testing.T) {
	p, err := loadYAML(`
name: service-gpus
services:
  test:
    image: redis:alpine
    gpus:
      - driver: nvidia
      - driver: 3dfx
        device_ids: ["voodoo2"]
        capabilities: ["directX"]
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test"].Gpus, []types.DeviceRequest{
		{
			Driver: "nvidia",
			Count:  -1,
		},
		{
			Capabilities: []string{"directX"},
			Driver:       "3dfx",
			IDs:          []string{"voodoo2"},
		},
	})
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

func TestEmptyFile(t *testing.T) {
	_, err := LoadWithContext(context.TODO(),
		types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{
				{
					Filename: filepath.Join("testdata", "empty.yaml"),
				},
			},
		})
	assert.Error(t, err, "empty compose file")
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
		Services: types.Services{
			"test": {
				Name: "test",
				EnvFiles: []types.EnvFile{
					{Path: file.Name(), Required: true},
				},
			},
		},
	}
	p, err = p.WithServicesEnvironmentResolved(false)
	assert.NilError(t, err)
	service, err := p.GetService("test")
	assert.NilError(t, err)
	assert.Equal(t, "YES", *service.Environment["HALLO"])
}

func TestLoadServiceWithLabelFile(t *testing.T) {
	file, err := os.CreateTemp("", "test-compose-go")
	assert.NilError(t, err)
	defer os.Remove(file.Name())

	_, err = file.Write([]byte("MY_LABEL=MY_VALUE"))
	assert.NilError(t, err)

	p := &types.Project{
		Services: types.Services{
			"test": {
				Name: "test",
				LabelFiles: []string{
					file.Name(),
				},
			},
		},
	}
	p, err = p.WithServicesLabelsResolved(false)
	assert.NilError(t, err)
	service, err := p.GetService("test")
	assert.NilError(t, err)
	assert.Equal(t, "MY_VALUE", service.Labels["MY_LABEL"])
}

func TestLoadServiceWithLabelFile_NotExists(t *testing.T) {
	p := &types.Project{
		Services: types.Services{
			"test": {
				Name: "test",
				LabelFiles: []string{
					"test",
				},
			},
		},
	}
	_, err := p.WithServicesLabelsResolved(false)
	assert.ErrorContains(t, err, "label file test not found")
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
	assert.Check(t, *actual.Services["test"].Init)
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
	assert.Equal(t, filepath.Join(os.Getenv("PWD"), "value1"), sshValue)
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
        - default
`)
	assert.NilError(t, err)
	svc, err := actual.GetService("test")
	assert.NilError(t, err)

	sshValue, err := svc.Build.SSH.Get("key1")
	assert.NilError(t, err)
	assert.Equal(t, filepath.Join(os.Getenv("PWD"), "value1"), sshValue)

	sshValue, err = svc.Build.SSH.Get("key2")
	assert.NilError(t, err)
	assert.Equal(t, filepath.Join(os.Getenv("PWD"), "value2"), sshValue)

	sshValue, err = svc.Build.SSH.Get("default")
	assert.NilError(t, err)
	assert.Equal(t, "", sshValue)
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

		actual, err := LoadWithContext(context.TODO(), configDetails)
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

		actual, err := LoadWithContext(context.TODO(), configDetails)
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

	t.Run("project name override", func(t *testing.T) {
		yaml := `
name: another_name
services:
  web:
    image: web
    container_name: ${COMPOSE_PROJECT_NAME}-web
`
		configDetails := buildConfigDetails(yaml, map[string]string{})

		actual, err := LoadWithContext(context.TODO(), configDetails, withProjectName("interpolated", true))
		assert.NilError(t, err)
		svc, err := actual.GetService("web")
		assert.NilError(t, err)
		assert.Equal(t, "interpolated-web", svc.ContainerName)
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

	project, err := LoadWithContext(context.TODO(), configDetails)
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

	project, err := LoadWithContext(context.TODO(), configDetails)
	assert.NilError(t, err)
	assert.Equal(t, project.Services["extension"].Name, "extension")
	assert.Equal(t, project.Services["extension"].Extensions["x-foo"], "bar")
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
		"foo": {
			Name:        "foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
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
		"foo": {
			Name:        "foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
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
    volumes:
      - x-volume:/dev/null
    configs:
      - x-config
    secrets:
      - x-secret
    networks:
      - x-network
volumes:
  x-volume:
secrets:
  x-secret:
    external: true
networks:
  x-network:
configs:
  x-config:
    external: true
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.ServiceNames(), []string{"x-foo"})
	assert.DeepEqual(t, p.SecretNames(), []string{"x-secret"})
	assert.DeepEqual(t, p.VolumeNames(), []string{"x-volume"})
	assert.DeepEqual(t, p.ConfigNames(), []string{"x-config"})
	assert.DeepEqual(t, p.NetworkNames(), []string{"x-network"})
}

func TestLoadWithInclude(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	p, err := LoadWithContext(context.TODO(), buildConfigDetails(`
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
		"foo": {
			Name:        "foo",
			Image:       "busybox",
			Environment: types.MappingWithEquals{},
			DependsOn:   types.DependsOnConfig{"imported": {Condition: "service_started", Required: true}},
		},
		"imported": {
			Name:          "imported",
			ContainerName: "extends", // as defined by ./testdata/subdir/extra.env
			Environment:   types.MappingWithEquals{"SOURCE": strPtr("extends")},
			EnvFiles: []types.EnvFile{
				{
					Path:     filepath.Join(workingDir, "testdata", "subdir", "extra.env"),
					Required: true,
				},
			},
			Image: "nginx",
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
	/* TODO(ndeloof) restore support for include tracking
	assert.DeepEqual(t, p.IncludeReferences, map[string][]types.IncludeConfig{
		filepath.Join(workingDir, "filename0.yml"): {
			{
				Path:             []string{filepath.Join(workingDir, "testdata", "subdir", "compose-test-extends-imported.yaml")},
				ProjectDirectory: workingDir,
				EnvFile:          []string{filepath.Join(workingDir, "testdata", "subdir", "extra.env")},
			},
		},
	})
	*/

	p, err = LoadWithContext(context.TODO(), buildConfigDetails(`
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
	imported, err := p.GetService("imported")
	assert.NilError(t, err)
	assert.Equal(t, imported.ContainerName, "override")
}

func TestLoadWithIncludeCycle(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	_, err = LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir: filepath.Join(workingDir, "testdata"),
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join(workingDir, "testdata", "compose-include-cycle.yaml"),
			},
		},
	})
	assert.Check(t, strings.HasPrefix(err.Error(), "include cycle detected"))
}

func TestLoadWithIncludeOverride(t *testing.T) {
	p, err := LoadWithContext(context.TODO(), buildConfigDetailsMultipleFiles(nil, `
name: 'test-include-override'

include:
  - ./testdata/subdir/compose-test-extends-imported.yaml
`,
		`
# override
services:
  imported:
    image: overridden
`), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["imported"].Image, "overridden")
}

func TestLoadDependsOnCycle(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	_, err = LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir: filepath.Join(workingDir, "testdata"),
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join(workingDir, "testdata", "compose-depends-on-cycle.yaml"),
			},
		},
	})
	assert.Error(t, err, "dependency cycle detected: service1 -> service2 -> service3 -> service1", err)
}

func TestLoadDependsOnSelf(t *testing.T) {
	workingDir, err := os.Getwd()
	assert.NilError(t, err)
	_, err = LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir: filepath.Join(workingDir, "testdata"),
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join(workingDir, "testdata", "compose-depends-on-self.yaml"),
			},
		},
	})
	assert.Error(t, err, "dependency cycle detected: service1 -> service1", err)
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
		"foo": {
			Name:        "foo",
			Image:       "nginx",
			Environment: types.MappingWithEquals{},
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

func (c customLoader) path(s string) string {
	return filepath.Join("testdata", c.prefix, s[len(c.prefix)+1:])
}

func (c customLoader) Load(_ context.Context, s string) (string, error) {
	path := c.path(s)
	_, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

func (c customLoader) Dir(s string) string {
	return filepath.Dir(c.path(s))
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
		"foo": {
			Name:        "foo",
			Image:       "foo",
			Environment: types.MappingWithEquals{"FOO": strPtr("BAR")},
			EnvFiles: []types.EnvFile{
				{
					Path:     filepath.Join(config.WorkingDir, "testdata", "remote", "env"),
					Required: true,
				},
			},
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
	assert.Equal(t, project.Services["test"].Image, "nginx:override")
}

func TestLoadDevelopConfig(t *testing.T) {
	project, err := LoadWithContext(context.TODO(), buildConfigDetails(`
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
          x-initialSync: true
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
          target: /etc/nginx/proxy.conf
`, nil), func(options *Options) {
		options.ResolvePaths = false
		options.SkipValidation = true
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
				Extensions: types.Extensions{
					"x-initialSync": true,
				},
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
				Target: "/etc/nginx/proxy.conf",
			},
		},
	})
}

func TestBadDevelopConfig(t *testing.T) {
	_, err := LoadWithContext(context.TODO(), buildConfigDetails(`
name: load-develop
services:
  frontend:
    image: example/webapp
    build: ./webapp
    develop:
      watch:
        # sync static content
        - path: ./webapp/html
          target: /var/www
          ignore:
            - node_modules/

`, nil), func(options *Options) {
		options.ResolvePaths = false
	})
	assert.ErrorContains(t, err, "services.frontend.develop.watch.0 missing property 'action'")
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

func TestLoadEmptyContent(t *testing.T) {
	yaml := `name: load-multi-docs
services:
  test:
    image: nginx:latest`
	tmpPath := filepath.Join(t.TempDir(), "docker-compose.yaml")
	if err := os.WriteFile(tmpPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temporary file: %s", err)
	}
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: tmpPath,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLoadEmptyContent_MissingProject(t *testing.T) {
	yaml := `
services:
  test:
    image: nginx:latest`
	tmpPath := filepath.Join(t.TempDir(), "docker-compose.yaml")
	if err := os.WriteFile(tmpPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("failed to write temporary file: %s", err)
	}
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: tmpPath,
			},
		},
	})
	assert.ErrorContains(t, err, "project name must not be empty")
}

func TestLoadUnitBytes(t *testing.T) {
	project, err := loadYAML(`
name: load-unit-bytes
services:
  test1:
    image: test
    memswap_limit: -1
  test2:
    image: test
    memswap_limit: 640kb
`)
	assert.NilError(t, err)
	test1 := project.Services["test1"]
	assert.NilError(t, err)
	assert.Equal(t, test1.MemSwapLimit, types.UnitBytes(-1))
	test2 := project.Services["test2"]
	assert.NilError(t, err)
	assert.Equal(t, test2.MemSwapLimit, types.UnitBytes(640*1024))
}

func TestBuildUlimits(t *testing.T) {
	yaml := `
name: test-build-ulimits
services:
  test:
    build:
      context: .
      ulimits:
        nproc: 65535
        nofile:
          soft: 20000
          hard: 40000
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	})
	assert.NilError(t, err)
	test := p.Services["test"]
	assert.DeepEqual(t, test.Build.Ulimits, map[string]*types.UlimitsConfig{
		"nproc":  {Single: 65535},
		"nofile": {Soft: 20000, Hard: 40000},
	})
}

func TestServiceNameWithDots(t *testing.T) {
	yaml := `
name: test-service-name-with-dots
services:
  test.a.b.c:
    image: foo
    ports:
      - "5432"
`
	_, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	})
	assert.NilError(t, err)
}

func TestLoadProjectName(t *testing.T) {
	const projectName = "testproject"

	tests := []struct {
		name    string
		env     map[string]string
		options func(*Options)
		wantErr string
	}{
		{
			name:    "default",
			options: func(_ *Options) {},
			wantErr: "project name must not be empty",
		},
		{
			name:    "project name from environment",
			env:     map[string]string{"COMPOSE_PROJECT_NAME": projectName},
			options: func(_ *Options) {},
			wantErr: "project name must not be empty",
		},
		{
			name:    "project name from options, not imperatively set; no env",
			options: withProjectName(projectName, false),
		},
		{
			name:    "project name from options, imperatively set; no env",
			options: withProjectName(projectName, true),
		},
		{
			name:    "project name from options, not imperatively set; empty env",
			env:     map[string]string{},
			options: withProjectName(projectName, false),
		},
		{
			name:    "project name from options, imperatively set; empty env",
			env:     map[string]string{},
			options: withProjectName(projectName, true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, err := LoadWithContext(context.Background(), types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{{
					Content: []byte(`
services:
  web:
    image: web
    container_name: ${COMPOSE_PROJECT_NAME}-web`),
				}},
				Environment: tt.env,
			}, tt.options)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, project.Name, projectName)
			assert.Equal(t, project.Environment["COMPOSE_PROJECT_NAME"], projectName)
			assert.Equal(t, project.Services["web"].ContainerName, projectName+"-web")
		})
	}
}

func withProjectName(projectName string, imperativelySet bool) func(*Options) {
	return func(opts *Options) {
		opts.SetProjectName(projectName, imperativelySet)
	}
}

func TestKnowExtensions(t *testing.T) {
	yaml := `
name: test-know-extensions
services:
  test:
    image: foo
    x-magic:
      foo: bar
`
	type Magic struct {
		Foo string
	}

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	}, func(options *Options) {
		options.KnownExtensions = map[string]any{
			"x-magic": Magic{},
		}
	})
	assert.NilError(t, err)
	x := p.Services["test"].Extensions["x-magic"]
	magic, ok := x.(Magic)
	assert.Check(t, ok)
	assert.Equal(t, magic.Foo, "bar")
}

func TestLoadWithEmptyFile(t *testing.T) {
	yaml := `
name: test-with-empty-file
services:
  test:
    image: foo
`

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "(inlined)",
				Content:  []byte(yaml),
			},
			{
				Filename: "(override)",
				Content:  []byte(""),
			},
		},
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Name, "test-with-empty-file")
}

func TestNamedPort(t *testing.T) {
	yaml := `
name: test-named-ports
services:
  test:
    image: foo
    ports:
      - name: http
        published: 8080
        target: 80
      - name: https
        published: 8083
        target: 443
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	})
	assert.NilError(t, err)
	ports := p.Services["test"].Ports
	assert.Equal(t, ports[0].Name, "http")
	assert.Equal(t, ports[1].Name, "https")
}

func TestAppProtocol(t *testing.T) {
	yaml := `
name: test-named-ports
services:
  test:
    image: foo
    ports:
      - published: 8080
        target: 80
        protocol: tcp
        app_protocol: http
      - published: 8083
        target: 443
        protocol: tcp
        app_protocol: https
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	})
	assert.NilError(t, err)
	ports := p.Services["test"].Ports
	assert.Equal(t, ports[0].AppProtocol, "http")
	assert.Equal(t, ports[1].AppProtocol, "https")
}

func TestBuildEntitlements(t *testing.T) {
	yaml := `
name: test-build-entitlements
services:
  test:
    build:
      context: .
      entitlements:
        - network.host
        - security.insecure
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content: []byte(yaml),
			},
		},
	})
	assert.NilError(t, err)
	build := p.Services["test"].Build
	assert.DeepEqual(t, build.Entitlements, []string{"network.host", "security.insecure"})
}

func TestLoadSecretEnvironment(t *testing.T) {
	config, err := loadYAMLWithEnv(`
name: load-secret-environment
configs:
  config:
    environment: GA
secrets:
  secret:
    environment: MEU
`, map[string]string{
		"GA":  "BU",
		"MEU": "Shadoks",
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, config.Configs, types.Configs{
		"config": {
			Environment: "GA",
			Content:     "BU",
		},
	}, cmpopts.IgnoreUnexported(types.ConfigObjConfig{}))
	assert.DeepEqual(t, config.Secrets, types.Secrets{
		"secret": {
			Environment: "MEU",
			Content:     "Shadoks",
		},
	}, cmpopts.IgnoreUnexported(types.SecretConfig{}))
}

func TestLoadDeviceMapping(t *testing.T) {
	config, err := loadYAML(`
name: load-device-mapping
services:
  test:
    devices:
      - /dev/source:/dev/target:permissions
      - /dev/single
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, config.Services["test"].Devices, []types.DeviceMapping{
		{
			Source:      "/dev/source",
			Target:      "/dev/target",
			Permissions: "permissions",
		},
		{
			Source:      "/dev/single",
			Target:      "/dev/single",
			Permissions: "rwm",
		},
	})
}

func TestLoadDeviceMappingLongSyntax(t *testing.T) {
	config, err := loadYAML(`
name: load-device-mapping-long-syntax
services:
  test:
    devices:
      - source: /dev/source
        target: /dev/target
        permissions: permissions
        x-foo: bar
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, config.Services["test"].Devices, []types.DeviceMapping{
		{
			Source:      "/dev/source",
			Target:      "/dev/target",
			Permissions: "permissions",
			Extensions: map[string]any{
				"x-foo": "bar",
			},
		},
	})
}

func TestLoadExtraHostsRepeated(t *testing.T) {
	p, err := loadYAML(`
name: load-extra-hosts
services:
  test:
    extra_hosts:
      - "myhost=0.0.0.1,0.0.0.2"
`)
	assert.NilError(t, err)
	hosts := p.Services["test"].ExtraHosts
	assert.DeepEqual(t, hosts, types.HostsList{
		"myhost": []string{"0.0.0.1", "0.0.0.2"},
	})
}

func TestLoadExtraHostsLongSyntax(t *testing.T) {
	p, err := loadYAML(`
name: load-extra-hosts
services:
  test:
    extra_hosts:
      myhost:
        - "0.0.0.1"
        - "0.0.0.2"
`)
	assert.NilError(t, err)
	hosts := p.Services["test"].ExtraHosts
	assert.DeepEqual(t, hosts, types.HostsList{
		"myhost": []string{"0.0.0.1", "0.0.0.2"},
	})
}

func TestLoadDependsOnX(t *testing.T) {
	p, err := loadYAML(`
name: load-depends-on-x
services:
  test:
    image: test
    depends_on:
      - x-foo
  x-foo:
    image: foo
`)
	assert.NilError(t, err)
	test := p.Services["test"]
	assert.DeepEqual(t, test.DependsOn, types.DependsOnConfig{
		"x-foo": types.ServiceDependency{
			Condition: types.ServiceConditionStarted,
			Required:  true,
		},
	})
}

func TestLoadDeviceReservation(t *testing.T) {
	config, err := loadYAML(`
name: load-device-reservation
services:
  test:
    deploy:
      resources:
        reservations:
          devices:
            - driver: richard_feynman
              capabilities: ["quantic"]
              count: all
              options:
                q_bits: 42
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, config.Services["test"].Deploy.Resources.Reservations.Devices, []types.DeviceRequest{
		{
			Capabilities: []string{"quantic"},
			Driver:       "richard_feynman",
			Count:        -1,
			Options: types.Mapping{
				"q_bits": "42",
			},
		},
	})
}

func TestLoadServiceHooks(t *testing.T) {
	p, err := loadYAML(`
name: load-service-hooks
services:
  test:
    post_start:
      - command: echo start
        user: root
        privileged: true
        working_dir: /
        environment:
          - FOO=BAR
    pre_stop:
      - command: echo stop
        user: root
        working_dir: /
        environment:
          FOO: BAR

`)
	assert.NilError(t, err)
	start := p.Services["test"].PostStart
	assert.DeepEqual(t, start, []types.ServiceHook{
		{
			Command:    types.ShellCommand{"echo", "start"},
			User:       "root",
			Privileged: true,
			WorkingDir: "/",
			Environment: types.MappingWithEquals{
				"FOO": strPtr("BAR"),
			},
		},
	})
	stop := p.Services["test"].PreStop
	assert.DeepEqual(t, stop, []types.ServiceHook{
		{
			Command:    types.ShellCommand{"echo", "stop"},
			User:       "root",
			WorkingDir: "/",
			Environment: types.MappingWithEquals{
				"FOO": strPtr("BAR"),
			},
		},
	})
}

func TestOmitEmptyDNS(t *testing.T) {
	p, err := loadYAML(`
name: load-empty-dsn
services:
  test:
    dns: ${UNSET_VAR}
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["test"].DNS), 0)
}

func TestAllGPUS(t *testing.T) {
	p, err := loadYAML(`
name: load-all-gpus
services:
  test:
    gpus: all
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["test"].Gpus), 1)
	assert.Equal(t, p.Services["test"].Gpus[0].Count, types.DeviceCount(-1))
}

func TestGwPriority(t *testing.T) {
	p, err := loadYAML(`
name: load-gw_priority
services:
  test:
    networks:
      test:
        gw_priority: 42
`)
	assert.NilError(t, err)
	assert.Equal(t, p.Services["test"].Networks["test"].GatewayPriority, 42)
}

func TestPullRefresh(t *testing.T) {
	p, err := loadYAML(`
name: load-all-gpus
services:
  test:
    pull_policy: every_2d
`)
	assert.NilError(t, err)
	policy, duration, err := p.Services["test"].GetPullPolicy()
	assert.NilError(t, err)
	assert.Equal(t, policy, types.PullPolicyRefresh)
	assert.Equal(t, duration, 2*24*time.Hour)
}

func TestEnvironmentWhitespace(t *testing.T) {
	_, err := loadYAML(`
name: environment_whitespace
services:
  test:
    environment:
      - DEBUG = true
`)
	assert.Check(t, strings.Contains(err.Error(), "'services[test].environment' environment variable DEBUG  is declared with a trailing space"), err.Error())
}

func TestFileModeNumber(t *testing.T) {
	p, err := loadYAML(`
name: load-file-mode
services:
  test:
    secrets:
      - source: server-certificate
        target: server.cert
        mode: 0o440 
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["test"].Secrets), 1)
	assert.Equal(t, *p.Services["test"].Secrets[0].Mode, types.FileMode(0o440))
}

func TestFileModeString(t *testing.T) {
	p, err := loadYAML(`
name: load-file-mode
services:
  test:
    secrets:
      - source: server-certificate
        target: server.cert
        mode: "0440" 
`)
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["test"].Secrets), 1)
	assert.Equal(t, *p.Services["test"].Secrets[0].Mode, types.FileMode(0o440))
}

func TestServiceProvider(t *testing.T) {
	p, err := loadYAML(`
name: service-provider
services:
  test:
    provider:
      type: foo
      options:
        bar: zot
        strings:
          - foo
          - bar
        numbers:
          - 12
          - 34
        booleans:
          - true
          - false
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test"].Provider, &types.ServiceProviderConfig{
		Type: "foo",
		Options: types.MultiOptions{
			"bar":      []string{"zot"},
			"strings":  []string{"foo", "bar"},
			"numbers":  []string{"12", "34"},
			"booleans": []string{"true", "false"},
		},
	})

	_, err = loadYAML(`
name: service-provider
services:
  test:
    provider:
      options:
        bar: zot
        strings: foo
        numbers: 12
        booleans: true
`)
	assert.ErrorContains(t, err, "services.test.provider missing property 'type'")
}

func TestImageVolume(t *testing.T) {
	p, err := loadYAML(`
name: imageVolume
services:
  test:
    volumes:
      - type: image
        source: app/image
        target: /mnt/image
        image:
          subpath: /foo
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test"].Volumes[0], types.ServiceVolumeConfig{
		Type:   "image",
		Source: "app/image",
		Target: "/mnt/image",
		Image:  &types.ServiceVolumeImage{SubPath: "/foo"},
	})
}

func TestNpipeVolume(t *testing.T) {
	p, err := loadYAML(`
name: imageVolume
services:
  test:
    volumes:
      - type: npipe
        source: \\.\pipe\docker_engine
        target: \\.\pipe\docker_engine
        image:
          subpath: /foo
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test"].Volumes[0], types.ServiceVolumeConfig{
		Type:   "npipe",
		Source: "\\\\.\\pipe\\docker_engine",
		Target: "\\\\.\\pipe\\docker_engine",
		Image:  &types.ServiceVolumeImage{SubPath: "/foo"},
	})
}

func TestInterfaceName(t *testing.T) {
	p, err := loadYAML(`
name: interface-name
services:
  test:
    networks:
      test:
        interface_name: eth0
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test"].Networks["test"], &types.ServiceNetworkConfig{
		InterfaceName: "eth0",
	})
}

func TestModel(t *testing.T) {
	p, err := loadYAML(`
name: model
services:
  test_array:
    models:
      - foo

  test_mapping:
    models:
      foo:
        endpoint_var: MODEL_URL
        model_var: MODEL

models:
  foo:
    model: ai/model
    context_size: 1024
    runtime_flags: 
      - "--some-flag"
`)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Models["foo"], types.ModelConfig{
		Model:        "ai/model",
		ContextSize:  1024,
		RuntimeFlags: []string{"--some-flag"},
	})
	assert.DeepEqual(t, p.Services["test_array"].Models, map[string]*types.ServiceModelConfig{
		"foo": nil,
	})
	assert.DeepEqual(t, p.Services["test_mapping"].Models, map[string]*types.ServiceModelConfig{
		"foo": {
			EndpointVariable: "MODEL_URL",
			ModelVariable:    "MODEL",
		},
	})
	assert.DeepEqual(t, p.ModelNames(), []string{"foo"})
	assert.Check(t, utils.ArrayContains(p.ServicesWithModels(), []string{"test_array", "test_mapping"}), p.ServicesWithModels())
}

func TestAttestations(t *testing.T) {
	p, err := loadYAML(`
name: attestations
services:
  test:
    build:
      context: .
      provenance: mode=max
      sbom: true
`)
	assert.NilError(t, err)
	build := p.Services["test"].Build
	assert.Equal(t, build.Provenance, "mode=max")
	assert.Equal(t, build.SBOM, "true")
}
