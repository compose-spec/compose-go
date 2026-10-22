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

package errdefs

import (
	"errors"
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
)

// sentinels lists every category exposed by the package, so that each helper
// is checked against all of them and a confusion between two categories fails.
var sentinels = map[string]error{
	"ErrNotFound":     ErrNotFound,
	"ErrInvalid":      ErrInvalid,
	"ErrUnsupported":  ErrUnsupported,
	"ErrIncompatible": ErrIncompatible,
}

// helpers pairs each Is*Error helper with the sentinel it must recognise.
var helpers = map[string]struct {
	is       func(error) bool
	sentinel string
}{
	"IsNotFoundError":     {IsNotFoundError, "ErrNotFound"},
	"IsInvalidError":      {IsInvalidError, "ErrInvalid"},
	"IsUnsupportedError":  {IsUnsupportedError, "ErrUnsupported"},
	"IsIncompatibleError": {IsIncompatibleError, "ErrIncompatible"},
}

// TestIsHelpers locks the contract of the Is*Error helpers: each one reports
// true for its own sentinel, including when callers wrap it with %w or join it
// with other errors (which is how compose-go returns them), and false for nil,
// for unrelated errors and for every other category.
func TestIsHelpers(t *testing.T) {
	for helperName, h := range helpers {
		t.Run(helperName, func(t *testing.T) {
			own := sentinels[h.sentinel]

			t.Run("sentinel", func(t *testing.T) {
				assert.Assert(t, h.is(own))
			})
			t.Run("wrapped", func(t *testing.T) {
				assert.Assert(t, h.is(fmt.Errorf("context: %w", own)))
			})
			t.Run("wrapped twice", func(t *testing.T) {
				assert.Assert(t, h.is(fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", own))))
			})
			t.Run("joined", func(t *testing.T) {
				assert.Assert(t, h.is(errors.Join(errors.New("other"), own)))
			})
			t.Run("nil", func(t *testing.T) {
				assert.Assert(t, !h.is(nil))
			})
			t.Run("unrelated", func(t *testing.T) {
				assert.Assert(t, !h.is(errors.New("unrelated")))
			})
			t.Run("unrelated wrapped", func(t *testing.T) {
				assert.Assert(t, !h.is(fmt.Errorf("context: %w", errors.New("unrelated"))))
			})
			t.Run("same message without the sentinel", func(t *testing.T) {
				// categories are identified by identity, not by message
				assert.Assert(t, !h.is(errors.New(own.Error())))
			})
			for name, other := range sentinels {
				if name == h.sentinel {
					continue
				}
				t.Run("not "+name, func(t *testing.T) {
					assert.Assert(t, !h.is(other))
					assert.Assert(t, !h.is(fmt.Errorf("context: %w", other)))
				})
			}
		})
	}
}

// TestSentinelMessages locks the text of the sentinels: it is surfaced to end
// users as the tail of wrapped errors (e.g. "no such service: foo: not found"),
// so it is part of the observable behavior.
func TestSentinelMessages(t *testing.T) {
	for name, want := range map[string]string{
		"ErrNotFound":     "not found",
		"ErrInvalid":      "invalid compose project",
		"ErrUnsupported":  "unsupported attribute",
		"ErrIncompatible": "incompatible attribute",
	} {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, sentinels[name], want)
		})
	}
}
