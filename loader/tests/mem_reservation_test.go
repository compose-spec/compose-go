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

// The tests in this file lock the `mem_reservation` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/container_spec.md#mem_reservation
//   https://github.com/compose-spec/compose-spec/blob/main/11-extension.md#specifying-byte-values
//
// Spec: "`mem_reservation` configures a reservation on the amount of memory a
// container can allocate, set as a string expressing a byte value. When both
// are set, `mem_reservation` must be consistent with the `reservations.memory`
// attribute in the Deploy Specification."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// mem_reservation is part of the container specification, shared by services and
// jobs, and survives a YAML and JSON round trip as a number of bytes.
func TestMemReservation(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    mem_reservation: 512m
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    mem_reservation: 512m
`)

	expect := func(p *types.Project) {
		assert.Equal(t, p.Services["foo"].MemReservation, types.UnitBytes(512*1024*1024))
		assert.Equal(t, p.Jobs["foo"].MemReservation, types.UnitBytes(512*1024*1024))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

// A byte value is accepted in every notation of the specification, and
// canonicalizes into a number of bytes.
func TestMemReservationByteValues(t *testing.T) {
	loadsByteValueNotations(t, "mem_reservation")
}

// The value is subject to interpolation like any other, whatever its notation.
func TestMemReservationInterpolation(t *testing.T) {
	for name, tc := range map[string]struct {
		env  string
		want types.UnitBytes
	}{
		"a value with a unit": {"512m", 512 * 1024 * 1024},
		"a number of bytes":   {"1048576", 1048576},
	} {
		t.Run(name, func(t *testing.T) {
			p := loadWithEnv(t, serviceWith("mem_reservation", "${MEM_RESERVATION}"), map[string]string{"MEM_RESERVATION": tc.env})
			assert.Equal(t, p.Services["foo"].MemReservation, tc.want)
		})
	}

	t.Run("a default value", func(t *testing.T) {
		loadsAs(t, serviceWith("mem_reservation", "${MEM_RESERVATION:-1g}"), serviceWith("mem_reservation", `"1073741824"`))
	})
}

// A later file overrides the reservation of an earlier one, whatever the notation
// each of them uses.
func TestMemReservationOverride(t *testing.T) {
	loadFilesAs(t,
		[]string{
			serviceWith("mem_reservation", "256m"),
			"services:\n  foo:\n    mem_reservation: 1g\n",
		},
		serviceWith("mem_reservation", `"1073741824"`))

	loadFilesAs(t,
		[]string{
			serviceWith("mem_reservation", "1048576"),
			"services:\n  foo:\n    mem_reservation: 2m\n",
		},
		serviceWith("mem_reservation", `"2097152"`))
}

// A value that is not a byte value is rejected, with the attribute in the
// error.
func TestMemReservationInvalid(t *testing.T) {
	for name, tc := range map[string]struct{ value, message string }{
		"an unknown notation": {`abc`, "invalid size"},
		"an empty string":     {`""`, "invalid size"},
		"a boolean":           {`true`, "must be a integer or string"},
		"a list":              {`[512m]`, "must be a integer or string"},
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, serviceWith("mem_reservation", tc.value))
			assert.ErrorContains(t, err, "mem_reservation")
			assert.ErrorContains(t, err, tc.message)
		})
	}
}

// When both are set, mem_reservation and deploy.resources.reservations.memory must agree,
// whatever the notation each of them uses.
func TestMemReservationConsistentWithDeployReservation(t *testing.T) {
	deploy := func(memory string) string {
		return `
name: test
services:
  foo:
    image: alpine
    mem_reservation: 512m
    deploy:
      resources:
        reservations:
          memory: ` + memory + `
`
	}

	t.Run("same value", func(t *testing.T) {
		_, err := loadConsistent(t, deploy("512M"))
		assert.NilError(t, err)
	})

	t.Run("distinct values", func(t *testing.T) {
		_, err := loadConsistent(t, deploy("1g"))
		assert.ErrorContains(t, err, "services.foo")
		assert.ErrorContains(t, err, "can't set distinct values on 'mem_reservation' and 'deploy.resources.reservations.memory'")
	})
}
