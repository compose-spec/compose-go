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

func TestModels(t *testing.T) {
	p := load(t, `
name: test
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
	assert.DeepEqual(t, p.Models["foo"], types.ModelConfig{
		Model:        "ai/model",
		ContextSize:  1024,
		RuntimeFlags: []string{"--some-flag"},
	})
	assert.Assert(t, p.Services["test_array"].Models["foo"] == nil)
	assert.Equal(t, p.Services["test_mapping"].Models["foo"].EndpointVariable, "MODEL_URL")
	assert.Equal(t, p.Services["test_mapping"].Models["foo"].ModelVariable, "MODEL")
}
