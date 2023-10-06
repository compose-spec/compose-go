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
	"testing"

	"github.com/compose-spec/compose-go/types"
	"gotest.tools/v3/assert"
)

func TestParseYAMLFiles(t *testing.T) {
	model, err := loadYamlModel(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "test.yaml",
				Content: []byte(`
services:
  test:
    image: foo
    command: echo hello
    init: true
`),
			},
			{Filename: "override.yaml",
				Content: []byte(`
services:
  test:
    image: bar
    command: echo world
    init: false
`)}}}, &Options{})
	assert.NilError(t, err)
	assert.DeepEqual(t, model, map[string]interface{}{
		"services": map[string]interface{}{
			"test": map[string]interface{}{
				"image":   "bar",
				"command": "echo world",
				"init":    false,
			},
		},
	})
}
