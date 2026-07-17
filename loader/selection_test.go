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
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func unmarshalModel(t *testing.T, doc string) map[string]any {
	t.Helper()
	var model map[string]any
	err := yaml.Unmarshal([]byte(doc), &model)
	assert.NilError(t, err)
	return model
}

func servicesOf(t *testing.T, dict map[string]any) map[string]any {
	t.Helper()
	services, ok := dict["services"].(map[string]any)
	assert.Assert(t, ok, "dict[\"services\"] should be a map[string]any")
	return services
}

func TestSelectModelServices(t *testing.T) {
	t.Run("single service with no dependencies", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
  b:
    image: b
`)

		result, err := SelectModelServices(dict, []string{"a"})
		assert.NilError(t, err)

		services := servicesOf(t, result)
		assert.Assert(t, is.Len(services, 1))
		_, ok := services["a"]
		assert.Assert(t, ok, "a should remain")
		assert.Assert(t, is.Len(servicesOf(t, dict), 1), "selection mutates the dict in place")
	})

	t.Run("transitive closure through canonical depends_on", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
    depends_on:
      b:
        condition: service_started
        required: true
  b:
    image: b
    depends_on:
      c:
        condition: service_healthy
        required: true
  c:
    image: c
  d:
    image: d
`)

		_, err := SelectModelServices(dict, []string{"a"})
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 3))
		for _, name := range []string{"a", "b", "c"} {
			_, ok := services[name]
			assert.Assert(t, ok, "%s should be kept", name)
		}
		_, hasD := services["d"]
		assert.Assert(t, !hasD, "unrelated service d should have been dropped")
	})

	t.Run("multiple disjoint roots are all kept", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
    depends_on:
      b:
        condition: service_started
        required: true
  b:
    image: b
  c:
    image: c
    depends_on:
      d:
        condition: service_started
        required: true
  d:
    image: d
  e:
    image: e
`)

		_, err := SelectModelServices(dict, []string{"a", "c"})
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 4))
		for _, name := range []string{"a", "b", "c", "d"} {
			_, ok := services[name]
			assert.Assert(t, ok, "%s should be kept", name)
		}
		_, hasE := services["e"]
		assert.Assert(t, !hasE, "e should have been dropped")
	})

	t.Run("unknown root name errors without mutating the dict", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
  b:
    image: b
`)

		_, err := SelectModelServices(dict, []string{"bogus"})
		assert.ErrorContains(t, err, "no such service: bogus")

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 2), "dict must be unmodified on error")
		_, hasA := services["a"]
		_, hasB := services["b"]
		assert.Assert(t, hasA && hasB, "all original services should still be present")
	})

	t.Run("depends_on key not matching any service is ignored", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
    depends_on:
      "${VAR}":
        condition: service_started
        required: true
  b:
    image: b
`)

		_, err := SelectModelServices(dict, []string{"a"})
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 1))
		_, hasA := services["a"]
		assert.Assert(t, hasA, "a should be kept")
		_, hasVar := services["${VAR}"]
		assert.Assert(t, !hasVar, "${VAR} must not be added as a service")
	})

	t.Run("empty names leaves dict unchanged", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
  b:
    image: b
`)

		_, err := SelectModelServices(dict, nil)
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 2))
	})

	t.Run("missing services key errors without panic", func(t *testing.T) {
		dict := map[string]any{"name": "test"}

		_, err := SelectModelServices(dict, []string{"a"})
		assert.ErrorContains(t, err, "no such service: a")
	})

	t.Run("empty services map errors without panic", func(t *testing.T) {
		dict := map[string]any{"services": map[string]any{}}

		_, err := SelectModelServices(dict, []string{"a"})
		assert.ErrorContains(t, err, "no such service: a")
	})

	t.Run("duplicate names in input are handled without panic", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
  b:
    image: b
`)

		_, err := SelectModelServices(dict, []string{"a", "a", "a"})
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 1))
		_, hasA := services["a"]
		assert.Assert(t, hasA)
	})

	t.Run("dependency cycle terminates and keeps all members", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
    depends_on:
      b:
        condition: service_started
        required: true
  b:
    image: b
    depends_on:
      a:
        condition: service_started
        required: true
  c:
    image: c
`)

		_, err := SelectModelServices(dict, []string{"a"})
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 2))
		_, hasA := services["a"]
		_, hasB := services["b"]
		assert.Assert(t, hasA && hasB, "both members of the cycle should be kept")
	})

	t.Run("self-dependency terminates", func(t *testing.T) {
		dict := unmarshalModel(t, `
services:
  a:
    image: a
    depends_on:
      a:
        condition: service_started
        required: true
  b:
    image: b
`)

		_, err := SelectModelServices(dict, []string{"a"})
		assert.NilError(t, err)

		services := servicesOf(t, dict)
		assert.Assert(t, is.Len(services, 1))
		_, hasA := services["a"]
		assert.Assert(t, hasA)
	})
}
