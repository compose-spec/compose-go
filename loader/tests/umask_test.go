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

// The tests in this file lock the `umask` attribute:
//   https://github.com/compose-spec/compose-spec/blob/main/05-services.md#umask
//
// Spec: "`umask` sets the initial file mode creation mask (umask) of the
// process in the service container. [...] The value is an integer, a string,
// or `null`. A numeric value is usually written in octal: YAML 1.1 supports a
// leading `0` followed by octal digits (for example, `0777`), while YAML 1.2
// further supports an explicit `0o` or `0O` prefix. A string value is parsed
// the same way. [...] If `umask` is omitted or `null`, Compose does not set a
// mask. [...] `umask: 0` masks no permission bits."

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestUmask(t *testing.T) {
	p := load(t, `
name: test
services:
  foo:
    image: alpine
    umask: 077
jobs:
  foo:
    triggers:
      manual: true
    image: alpine
    umask: 077
`)

	expect := func(p *types.Project) {
		assert.DeepEqual(t, p.Services["foo"].Umask, ptr(uint32(0o77)))
		assert.DeepEqual(t, p.Jobs["foo"].Umask, ptr(uint32(0o77)))
	}
	expect(p)

	yamlP, jsonP := roundTrip(t, p)
	expect(yamlP)
	expect(jsonP)
}

func TestUmaskNotation(t *testing.T) {
	for name, tc := range map[string]struct {
		value string
		want  uint32
	}{
		"octal with a leading 0":        {`077`, 0o77},
		"octal with the 0o prefix":      {`0o77`, 0o77},
		"octal with the 0O prefix":      {`0O77`, 0o77},
		"decimal without a leading 0":   {`77`, 77},
		"zero":                          {`0`, 0},
		"octal string":                  {`"077"`, 0o77},
		"octal string with 0o prefix":   {`"0o77"`, 0o77},
		"decimal string":                {`"77"`, 77},
		"zero string":                   {`"0"`, 0},
		"highest permission bits mask":  {`0777`, 0o777},
		"setuid, setgid and sticky bit": {`07777`, 0o7777},
	} {
		t.Run(name, func(t *testing.T) {
			p := load(t, "name: test\nservices:\n  foo:\n    image: alpine\n    umask: "+tc.value+"\n")
			assert.DeepEqual(t, p.Services["foo"].Umask, ptr(tc.want))
		})
	}
}

// An explicit 0 masks no permission bit, which differs from no umask at all.
func TestUmaskZeroIsNotUnset(t *testing.T) {
	p := load(t, `
name: test
services:
  explicit:
    image: alpine
    umask: 0
  unset:
    image: alpine
    umask: null
  omitted:
    image: alpine
`)

	assert.DeepEqual(t, p.Services["explicit"].Umask, ptr(uint32(0)))
	assert.Assert(t, p.Services["unset"].Umask == nil)
	assert.Assert(t, p.Services["omitted"].Umask == nil)

	yamlP, jsonP := roundTrip(t, p)
	for _, rt := range []*types.Project{yamlP, jsonP} {
		assert.DeepEqual(t, rt.Services["explicit"].Umask, ptr(uint32(0)))
		assert.Assert(t, rt.Services["unset"].Umask == nil)
		assert.Assert(t, rt.Services["omitted"].Umask == nil)
	}
}

func TestUmaskInterpolation(t *testing.T) {
	for value, want := range map[string]uint32{
		"077":  0o77,
		"0o77": 0o77,
		"77":   77,
		"0":    0,
	} {
		t.Run(value, func(t *testing.T) {
			p := loadWithEnv(t, `
name: test
services:
  foo:
    image: alpine
    umask: ${UMASK}
`, map[string]string{"UMASK": value})
			assert.DeepEqual(t, p.Services["foo"].Umask, ptr(want))
		})
	}
}

func TestUmaskOverride(t *testing.T) {
	m := loadFilesMap(t, []string{
		"name: test\nservices:\n  foo:\n    image: alpine\n    umask: 022\n",
		"services:\n  foo:\n    umask: 077\n",
	})
	assert.Equal(t, m["services"].(map[string]any)["foo"].(map[string]any)["umask"], 0o77)
}

func TestUmaskInvalid(t *testing.T) {
	for name, value := range map[string]string{
		"negative":            `-1`,
		"not a number":        `abc`,
		"out of range":        `4294967296`,
		"out of range string": `"4294967296"`,
		"a boolean":           `true`,
		"a list":              `[077]`,
		"an empty string":     `""`,
		"a fractional part":   `0.5`,
	} {
		t.Run(name, func(t *testing.T) {
			err := loadErr(t, "name: test\nservices:\n  foo:\n    image: alpine\n    umask: "+value+"\n")
			assert.ErrorContains(t, err, "umask")
		})
	}
}

// A pre_start init container is a full container specification: once the
// inheritance is resolved at load time it carries the umask of its service
// unless it declares its own, an explicit 0 included.
func TestUmaskPreStartInheritance(t *testing.T) {
	fullLoad := func(yaml string) *types.Project {
		t.Helper()
		p, err := loader.LoadWithContext(t.Context(), types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(yaml)}},
			Environment: map[string]string{},
		})
		assert.NilError(t, err)
		return p
	}

	p := fullLoad(`
name: test
services:
  foo:
    image: alpine
    umask: 077
    pre_start:
      - command: ["true"]
      - command: ["true"]
        umask: 0
`)

	expect := func(p *types.Project) {
		hooks := p.Services["foo"].PreStart
		assert.Equal(t, len(hooks), 2)
		assert.DeepEqual(t, hooks[0].Umask, ptr(uint32(0o77)))
		assert.DeepEqual(t, hooks[1].Umask, ptr(uint32(0)))
	}
	expect(p)

	out, err := p.MarshalYAML()
	assert.NilError(t, err)
	expect(fullLoad(string(out)))
}
