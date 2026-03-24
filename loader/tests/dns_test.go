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

func TestDNS(t *testing.T) {
	p := load(t, `
name: test
services:
  list:
    image: alpine
    dns:
      - 8.8.8.8
      - 9.9.9.9
  string:
    image: alpine
    dns: 8.8.8.8
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["list"].DNS, types.StringList{"8.8.8.8", "9.9.9.9"})
		assert.DeepEqual(t, p.Services["string"].DNS, types.StringList{"8.8.8.8"})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDNSSearch(t *testing.T) {
	p := load(t, `
name: test
services:
  list:
    image: alpine
    dns_search:
      - dc1.example.com
      - dc2.example.com
  string:
    image: alpine
    dns_search: example.com
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["list"].DNSSearch, types.StringList{"dc1.example.com", "dc2.example.com"})
		assert.DeepEqual(t, p.Services["string"].DNSSearch, types.StringList{"example.com"})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestDNSOmitEmpty(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    dns: ${UNSET_VAR}
`)
	assert.Equal(t, len(p.Services["foo"].DNS), 0)
}
