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

func TestExtraHostsMap(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    extra_hosts:
      alpha: "50.31.209.229"
      zulu: "162.242.195.82"
`)
	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].ExtraHosts, types.HostsList{
			"alpha": []string{"50.31.209.229"},
			"zulu":  []string{"162.242.195.82"},
		})
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestExtraHostsList(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    extra_hosts:
      - "alpha:50.31.209.229"
      - "zulu:127.0.0.2"
      - "zulu:ff02::1"
`)
	assert.DeepEqual(t, p.Services["foo"].ExtraHosts, types.HostsList{
		"alpha": []string{"50.31.209.229"},
		"zulu":  []string{"127.0.0.2", "ff02::1"},
	})
}

func TestExtraHostsRepeated(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    extra_hosts:
      - "myhost=0.0.0.1,0.0.0.2"
`)
	assert.DeepEqual(t, p.Services["foo"].ExtraHosts, types.HostsList{
		"myhost": []string{"0.0.0.1", "0.0.0.2"},
	})
}

func TestExtraHostsLongSyntax(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    extra_hosts:
      myhost:
        - "0.0.0.1"
        - "0.0.0.2"
`)
	assert.DeepEqual(t, p.Services["foo"].ExtraHosts, types.HostsList{
		"myhost": []string{"0.0.0.1", "0.0.0.2"},
	})
}
