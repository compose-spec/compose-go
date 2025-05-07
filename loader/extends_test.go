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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestExtends(t *testing.T) {
	yaml := `
name: test-extends
services:
  test1:
    extends:
      file: testdata/extends/base.yaml
      service: base
    hostname: test1

  test2:
    extends:
      file: testdata/extends/base.yaml
      service: base
    hostname: test2

  test3:
    extends:
      file: testdata/extends/base.yaml
      service: another
    hostname: test3
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	extendsCount := 0
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	}, func(options *Options) {
		options.ResolvePaths = false
		options.Listeners = []Listener{
			func(event string, _ map[string]any) {
				if event == "extends" {
					extendsCount++
				}
			},
		}
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["test1"].Hostname, "test1")
	assert.Equal(t, p.Services["test2"].Hostname, "test2")
	assert.Equal(t, p.Services["test3"].Hostname, "test3")
	assert.Equal(t, extendsCount, 4)
}

func TestExtendsPort(t *testing.T) {
	yaml := `
name: test-extends-port
services:
  test:
    image: test
    extends: 
      file: testdata/extends/base.yaml
      service: with-port
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["test"].Ports[0].Target, uint32(8000))
}

func TestExtendsUlimits(t *testing.T) {
	yaml := `
name: test-extends
services:
  test:
    extends:
      file: testdata/extends/base.yaml
      service: withUlimits
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["test"].Ulimits["nproc"].Single, 65535)
}

func TestExtendsRelativePath(t *testing.T) {
	yaml := `
name: test-extends-port
services:
  test:
    extends: 
      file: testdata/extends/base.yaml
      service: with-build
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	}, func(options *Options) {
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	assert.Equal(t, p.Services["test"].Build.Context, filepath.Join("testdata", "extends"))
}

func TestExtendsNil(t *testing.T) {
	yaml := `
name: test-extends-port
services:
  test:
    image: test
    extends:
      file: testdata/extends/base.yaml
      service: nil
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	_, err = LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	}, func(options *Options) {
		options.ResolvePaths = false
		options.SkipValidation = true
	})
	assert.NilError(t, err)
}

func TestExtendsWihtMissingService(t *testing.T) {
	yaml := `
name: test-extends-port
services:
  test:
    image: test
    extends:
      file: testdata/extends/base.yaml
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	_, err = LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	}, func(options *Options) {
		options.ResolvePaths = false
		options.SkipValidation = true
	})
	assert.Error(t, err, "extends.test.service is required")
}

func TestIncludeWithExtends(t *testing.T) {
	yaml := `
name: test-include-with-extends
include: 
  - testdata/extends/nested.yaml
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Content:  []byte(yaml),
				Filename: "(inline)",
			},
		},
		WorkingDir: abs,
	}, func(options *Options) {
		options.ResolvePaths = false
		options.SkipValidation = true
	})
	assert.NilError(t, err)
	assert.Check(t, p.Services["with-build"].Build != nil)
}

func TestExtendsPortOverride(t *testing.T) {
	yaml := `
name: test-extends-port
services:
  test:
    extends:
      file: testdata/extends/ports.yaml
      service: test
`
	abs, err := filepath.Abs(".")
	assert.NilError(t, err)

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "testdata/extends/ports.yaml",
			},
			{
				Content:  []byte(yaml),
				Filename: "(override)",
			},
		},
		WorkingDir: abs,
	}, func(options *Options) {
		options.ResolvePaths = false
		options.SkipValidation = true
	})
	assert.NilError(t, err)
	assert.Equal(t, len(p.Services["test"].Ports), 1)
}

func TestLoadExtendsSameFile(t *testing.T) {
	tmpdir := t.TempDir()

	aDir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.Mkdir(aDir, 0o700))
	aYAML := `
services:
  base:
    build:
      context: ..
  service:
    extends: base
    build:
      target: target
`

	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "sub", "compose.yaml"), []byte(aYAML), 0o600))

	rootYAML := `
services:
  out-base:
    extends:
      file: sub/compose.yaml
      service: base
  out-service:
    extends:
      file: sub/compose.yaml
      service: service
`

	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "compose.yaml"), []byte(rootYAML), 0o600))

	extendsCount := 0
	actual, err := LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(tmpdir, "compose.yaml"),
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
		options.SetProjectName("project", true)
		options.Listeners = []Listener{
			func(event string, _ map[string]any) {
				if event == "extends" {
					extendsCount++
				}
			},
		}
	})
	assert.NilError(t, err)
	assert.Assert(t, is.Len(actual.Services, 2))

	svcA, err := actual.GetService("out-base")
	assert.NilError(t, err)
	assert.Equal(t, svcA.Build.Context, tmpdir)

	svcB, err := actual.GetService("out-service")
	assert.NilError(t, err)
	assert.Equal(t, svcB.Build.Context, tmpdir)

	assert.Equal(t, extendsCount, 3)
}

func TestExtendsWithServiceRef(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "volumes_from",
			yaml: `
name: test-extends_with_volumes_from
services:
  bar: 
    extends:
      file: ./testdata/extends/depends_on.yaml
      service: with_volumes_from
`,
			wantErr: `service "bar" depends on undefined service "zot"`,
		},
		{
			name: "depends_on",
			yaml: `
name: test-extends_with_depends_on
services:
  bar:
    extends:
      file: ./testdata/extends/depends_on.yaml
      service: with_depends_on
`,
			wantErr: `service "bar" depends on undefined service "zot"`,
		},
		{
			name: "shared ipc",
			yaml: `
name: test-extends_with_shared_ipc
services:
  bar:
    extends:
      file: ./testdata/extends/depends_on.yaml
      service: with_ipc
`,
			wantErr: `service "bar" depends on undefined service "zot"`,
		},
		{
			name: "shared network_mode",
			yaml: `
name: test-extends_with_shared_network_mode
services:
  bar:
    extends:
      file: ./testdata/extends/depends_on.yaml
      service: with_network_mode
`,
			wantErr: `service "bar" depends on undefined service "zot"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadWithContext(context.Background(), types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{{
					Content: []byte(tt.yaml),
				}},
			})
			if tt.wantErr == "" {
				assert.NilError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
			// Do the same but with a local `zot` service matching the imported reference
			_, err = LoadWithContext(context.Background(), types.ConfigDetails{
				ConfigFiles: []types.ConfigFile{{
					Content: []byte(tt.yaml + `
  zot:
    image: zot
`),
				}},
			})
			assert.NilError(t, err)
		})
	}
}

func TestLoadExtendsDependsOn(t *testing.T) {
	yaml := `
name: extends-depends_on
services:
  service_a:
    image: a
    depends_on:
      - service_c
  
  service_b:
    extends: service_a
    depends_on: !override
      - service_a

  service_c:
    image: c`
	p, err := loadYAML(yaml)
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["service_b"].DependsOn, types.DependsOnConfig{
		"service_a": types.ServiceDependency{Condition: types.ServiceConditionStarted, Required: true},
	})
}

func TestLoadExtendsListener(t *testing.T) {
	yaml := `
  name: listener-extends
  services:
    foo:
      image: busybox
      extends: bar
    bar:
      image: alpine
      command: echo
      extends: wee
    wee:
      extends: last
      command: echo
    last:
      image: python`
	extendsCount := 0
	_, err := LoadWithContext(context.Background(), buildConfigDetails(yaml, nil), func(options *Options) {
		options.SkipConsistencyCheck = true
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.Listeners = []Listener{
			func(event string, _ map[string]any) {
				if event == "extends" {
					extendsCount++
				}
			},
		}
	})

	assert.NilError(t, err)
	assert.Equal(t, extendsCount, 3)
}

func TestLoadExtendsListenerMultipleFiles(t *testing.T) {
	tmpdir := t.TempDir()
	subDir := filepath.Join(tmpdir, "sub")
	assert.NilError(t, os.Mkdir(subDir, 0o700))
	subYAML := `
services:
  b:
    extends: c
    build:
      target: fake
  c:
    command: echo
`
	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "sub", "compose.yaml"), []byte(subYAML), 0o600))

	rootYAML := `
services:
  a:
    extends:
      file: ./sub/compose.yaml
      service: b
`
	assert.NilError(t, os.WriteFile(filepath.Join(tmpdir, "compose.yaml"), []byte(rootYAML), 0o600))

	extendsCount := 0
	_, err := LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: filepath.Join(tmpdir, "compose.yaml"),
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.SkipConsistencyCheck = true
		options.SetProjectName("project", true)
		options.Listeners = []Listener{
			func(event string, _ map[string]any) {
				if event == "extends" {
					extendsCount++
				}
			},
		}
	})
	assert.NilError(t, err)
	assert.Equal(t, extendsCount, 2)
}

func TestExtendsReset(t *testing.T) {
	yaml := `
name: test-extends-reset
services:
  test:
    extends:
      file: testdata/extends/reset.yaml
      service: base
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{
			Content:  []byte(yaml),
			Filename: "-",
		}},
	})
	assert.NilError(t, err)
	assert.Check(t, p.Services["test"].Command == nil)
}

func TestExtendsWithInterpolation(t *testing.T) {
	yaml := `
name: test-extends-with-interpolation
services:
  test:
    extends: { file: testdata/extends/interpolated.yaml, service: foo }
`
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{
			Content:  []byte(yaml),
			Filename: "-",
		}},
	})
	assert.NilError(t, err)
	assert.Check(t, p.Services["test"].Volumes[0].Source == "/dev/null")
}
