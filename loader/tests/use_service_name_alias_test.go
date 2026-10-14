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

// The tests in this file lock the `networks.use_service_name_alias` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#use_service_name_alias
//
// Spec: "`use_service_name_alias` [...] controls whether the service name is
// registered as an alias of the service's containers on that network. It
// defaults to `true`."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// An unset attribute stays unset (nil, not false): implementations must tell
// "unset" from `false`, and an unset value must not be serialized back.
func TestUseServiceNameAliasUnset(t *testing.T) {
	loadsAs(t, `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        aliases: [test-alias]
networks:
  shared:
`, `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        aliases: [test-alias]
networks:
  shared: {}
`)
}

// The attribute is read from the long `networks` syntax, for both values, and
// survives a YAML and JSON round trip.
func TestUseServiceNameAlias(t *testing.T) {
	p := load(t, `
name: test
services:
  disabled:
    image: alpine
    networks:
      shared:
        aliases: [project1-disabled]
        use_service_name_alias: false
  enabled:
    image: alpine
    networks:
      shared:
        use_service_name_alias: true
networks:
  shared:
    external: true
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["disabled"].Networks["shared"].UseServiceNameAlias, ptr(false))
		assert.DeepEqual(t, p.Services["enabled"].Networks["shared"].UseServiceNameAlias, ptr(true))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// The attribute is interpolated like any other boolean of the specification.
func TestUseServiceNameAliasInterpolation(t *testing.T) {
	p := loadWithEnv(t, `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        use_service_name_alias: ${USE_SERVICE_NAME_ALIAS}
networks:
  shared:
    external: true
`, map[string]string{"USE_SERVICE_NAME_ALIAS": "false"})
	assert.DeepEqual(t, p.Services["test"].Networks["shared"].UseServiceNameAlias, ptr(false))
}

// A value that is not a boolean is rejected by the interpolation cast, which
// reports the dotted model path (the decoder alone would report
// services[test].networks[shared]...).
func TestUseServiceNameAliasInvalid(t *testing.T) {
	err := loadErr(t, `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        use_service_name_alias: maybe
networks:
  shared:
    external: true
`)
	assert.ErrorContains(t, err, "services.test.networks.shared.use_service_name_alias")
	assert.ErrorContains(t, err, "invalid boolean: maybe")
}

// The attribute lives on the networks definition shared by services and jobs:
// a job accepts it, with the same boolean interpolation cast and the same
// round trip as a service.
func TestUseServiceNameAliasJob(t *testing.T) {
	p := load(t, `
name: test
jobs:
  migrate:
    image: alpine
    triggers:
      manual: true
    networks:
      shared:
        use_service_name_alias: false
networks:
  shared:
    external: true
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Jobs["migrate"].Networks["shared"].UseServiceNameAlias, ptr(false))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)

	err := loadErr(t, `
name: test
jobs:
  migrate:
    image: alpine
    triggers:
      manual: true
    networks:
      shared:
        use_service_name_alias: maybe
networks:
  shared:
    external: true
`)
	assert.ErrorContains(t, err, "jobs.migrate.networks.shared.use_service_name_alias")
	assert.ErrorContains(t, err, "invalid boolean: maybe")
}

// Consumers need not re-implement the default: ServiceNameAliasEnabled maps an
// unset attribute to true, whatever form the network entry took (short
// syntax and null entries load as a nil *ServiceNetworkConfig).
func TestServiceNameAliasEnabled(t *testing.T) {
	p := load(t, `
name: test
services:
  test:
    image: alpine
    networks:
      short:
      null_entry:
      unset:
        aliases: [test-alias]
      enabled:
        use_service_name_alias: true
      disabled:
        use_service_name_alias: false
networks:
  short:
  null_entry:
  unset:
  enabled:
  disabled:
`)
	networks := p.Services["test"].Networks
	for name, want := range map[string]bool{
		"short":      true,
		"null_entry": true,
		"unset":      true,
		"enabled":    true,
		"disabled":   false,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, networks[name].ServiceNameAliasEnabled(), want)
		})
	}
}

// Spec (PR #663): the attribute merges like any scalar across override files,
// so a later file can set it back to `true`, and a file that does not mention
// it leaves the previous value untouched.
func TestUseServiceNameAliasMerge(t *testing.T) {
	base := `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        use_service_name_alias: false
networks:
  shared:
    external: true
`
	t.Run("override sets it back to true", func(t *testing.T) {
		loadFilesAs(t, []string{base, `
services:
  test:
    networks:
      shared:
        use_service_name_alias: true
`}, `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        use_service_name_alias: true
networks:
  shared:
    external: true
`)
	})
	t.Run("override without it keeps false", func(t *testing.T) {
		loadFilesAs(t, []string{base, `
services:
  test:
    networks:
      shared:
        aliases: [test-alias]
`}, `
name: test
services:
  test:
    image: alpine
    networks:
      shared:
        aliases: [test-alias]
        use_service_name_alias: false
networks:
  shared:
    external: true
`)
	})
}
