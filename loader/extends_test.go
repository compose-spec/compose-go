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
	assert.DeepEqual(t, p.Services["test1"].Hostname, "test1")
	assert.Equal(t, p.Services["test2"].Hostname, "test2")
	assert.Equal(t, p.Services["test3"].Hostname, "test3")
}
