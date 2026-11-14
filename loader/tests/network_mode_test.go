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

// The tests in this file lock the `network_mode` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#network_mode
//
// Spec: "`network_mode` sets a service container's network mode."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/errdefs"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestNetworkMode(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    network_mode: "container:0cfeab0f748b"
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    network_mode: "container:0cfeab0f748b"
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].NetworkMode, "container:0cfeab0f748b")
		assert.Equal(t, p.Jobs["foo"].NetworkMode, "container:0cfeab0f748b")
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// `network_mode: service:<name>` makes the service depend on the one it shares the
// network of, so a target the project cannot run is reported as an undefined
// dependency, unless the dependency is explicitly optional: then the consistency
// check on network_mode itself names the service that declares the target, and
// tells a target disabled by profiles from an unknown one.
func TestNetworkModeServiceDisabledTarget(t *testing.T) {
	_, err := loadConsistent(t, `
name: test
services:
  web:
    image: alpine
    network_mode: service:db
    depends_on:
      db:
        condition: service_started
        required: false
  db:
    image: alpine
    profiles: [debug]
`)
	assert.ErrorContains(t, err, `service "web" declares disabled service "db" for network_mode`)
	assert.ErrorIs(t, err, errdefs.ErrInvalid)
}
