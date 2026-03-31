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

func TestSysctls(t *testing.T) {
	p := load(t, `
name: test
services:
  list:
    image: busybox
    sysctls:
      - net.core.somaxconn=1024
      - net.ipv4.tcp_syncookies=0
      - testing.one.one=
      - testing.one.two
  map:
    image: busybox
    sysctls:
      net.core.somaxconn: 1024
      net.ipv4.tcp_syncookies: 0
      testing.one.one: ""
      testing.one.two:
`)

	expect := func(p *types.Project) {
		expected := types.Mapping{
			"net.core.somaxconn":      "1024",
			"net.ipv4.tcp_syncookies": "0",
			"testing.one.one":         "",
			"testing.one.two":         "",
		}
		assert.DeepEqual(t, p.Services["list"].Sysctls, expected)
		assert.DeepEqual(t, p.Services["map"].Sysctls, expected)
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}
