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

package tests

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestGpus(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    gpus:
      - driver: nvidia
      - driver: 3dfx
        device_ids: ["voodoo2"]
        capabilities: ["directX"]
`)
	assert.DeepEqual(t, p.Services["test"].Gpus, []types.DeviceRequest{
		{Driver: "nvidia", Count: -1},
		{Capabilities: []string{"directX"}, Driver: "3dfx", IDs: []string{"voodoo2"}},
	})
}

func TestGpusAll(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    gpus: all
`)
	assert.Equal(t, len(p.Services["test"].Gpus), 1)
	assert.Equal(t, p.Services["test"].Gpus[0].Count, types.DeviceCount(-1))
}
