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
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

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
	imported, err := p.GetService("imported")
	assert.NilError(t, err)
	assert.Equal(t, imported.ContainerName, "override")

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
	assert.ErrorContains(t, err, "services.bar conflicts with imported resource", err)
}

func TestIncludeRelative(t *testing.T) {
	wd, err := filepath.Abs(filepath.Join("testdata", "include"))
	assert.NilError(t, err)
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join("testdata", "include", "compose.yaml"),
			},
		},
		WorkingDir: wd,
	}, func(options *Options) {
		options.projectName = "test-include-relative"
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	included := p.Services["included"]
	assert.Equal(t, included.Build.Context, ".")
	assert.Equal(t, included.Volumes[0].Source, ".")
}
