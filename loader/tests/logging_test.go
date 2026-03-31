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

func TestLogging(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    logging:
      driver: syslog
      options:
        syslog-address: "tcp://192.168.0.42:123"
`)
	expect := func(p *types.Project) {
		expected := &types.LoggingConfig{
			Driver:  "syslog",
			Options: map[string]string{"syslog-address": "tcp://192.168.0.42:123"},
		}
		assert.DeepEqual(t, p.Services["foo"].Logging, expected)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
