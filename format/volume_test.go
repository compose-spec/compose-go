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

package format

import (
	"fmt"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestParseVolumeAnonymousVolume(t *testing.T) {
	for _, path := range []string{"/path", "/path/foo"} {
		volume, err := ParseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path, Volume: &types.ServiceVolumeVolume{}}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeAnonymousVolumeWindows(t *testing.T) {
	for _, path := range []string{"C:\\path", "Z:\\path\\foo"} {
		volume, err := ParseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path, Volume: &types.ServiceVolumeVolume{}}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeTooManyColons(t *testing.T) {
	_, err := ParseVolume("/foo:/foo:ro:foo")
	assert.Error(t, err, "invalid spec: /foo:/foo:ro:foo: too many colons")
}

func TestParseVolumeShortVolumes(t *testing.T) {
	for _, path := range []string{".", "/a"} {
		volume, err := ParseVolume(path)
		expected := types.ServiceVolumeConfig{Type: "volume", Target: path}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeMissingSource(t *testing.T) {
	for _, spec := range []string{":foo", "/foo::ro"} {
		_, err := ParseVolume(spec)
		assert.ErrorContains(t, err, "empty section between colons")
	}
}

func TestParseVolumeBindMount(t *testing.T) {
	for _, path := range []string{"./foo", "~/thing", "../other", "/foo", "/home/user"} {
		volume, err := ParseVolume(path + ":/target")
		expected := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: path,
			Target: "/target",
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeRelativeBindMountWindows(t *testing.T) {
	for _, path := range []string{
		"./foo",
		"~/thing",
		"../other",
		"D:\\path", "/home/user",
	} {
		volume, err := ParseVolume(path + ":d:\\target")
		expected := types.ServiceVolumeConfig{
			Type:   "bind",
			Source: path,
			Target: "d:\\target",
			Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeWithBindOptions(t *testing.T) {
	volume, err := ParseVolume("/source:/target:slave")
	expected := types.ServiceVolumeConfig{
		Type:   "bind",
		Source: "/source",
		Target: "/target",
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			Propagation:    "slave",
		},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithBindOptionsSELinuxShared(t *testing.T) {
	volume, err := ParseVolume("/source:/target:ro,z")
	expected := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "/source",
		Target:   "/target",
		ReadOnly: true,
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			SELinux:        "z",
		},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithBindOptionsSELinuxPrivate(t *testing.T) {
	volume, err := ParseVolume("/source:/target:ro,Z")
	expected := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "/source",
		Target:   "/target",
		ReadOnly: true,
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			SELinux:        "Z",
		},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithBindOptionsWindows(t *testing.T) {
	volume, err := ParseVolume("C:\\source\\foo:D:\\target:ro,rprivate")
	expected := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "C:\\source\\foo",
		Target:   "D:\\target",
		ReadOnly: true,
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			Propagation:    "rprivate",
		},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithInvalidVolumeOptions(t *testing.T) {
	_, err := ParseVolume("name:/target:bogus")
	assert.NilError(t, err)
}

func TestParseVolumeWithVolumeOptions(t *testing.T) {
	volume, err := ParseVolume("name:/target:nocopy")
	expected := types.ServiceVolumeConfig{
		Type:   "volume",
		Source: "name",
		Target: "/target",
		Volume: &types.ServiceVolumeVolume{NoCopy: true},
	}
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestParseVolumeWithReadOnly(t *testing.T) {
	for _, path := range []string{"./foo", "/home/user"} {
		volume, err := ParseVolume(path + ":/target:ro")
		expected := types.ServiceVolumeConfig{
			Type:     "bind",
			Source:   path,
			Target:   "/target",
			ReadOnly: true,
			Bind:     &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeWithRW(t *testing.T) {
	for _, path := range []string{"./foo", "/home/user"} {
		volume, err := ParseVolume(path + ":/target:rw")
		expected := types.ServiceVolumeConfig{
			Type:     "bind",
			Source:   path,
			Target:   "/target",
			ReadOnly: false,
			Bind:     &types.ServiceVolumeBind{CreateHostPath: true},
		}
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(expected, volume))
	}
}

func TestParseVolumeWindowsNamedPipe(t *testing.T) {
	volume, err := ParseVolume(`\\.\pipe\docker_engine:\\.\pipe\inside`)
	assert.NilError(t, err)
	expected := types.ServiceVolumeConfig{
		Type:   "bind",
		Source: `\\.\pipe\docker_engine`,
		Target: `\\.\pipe\inside`,
		Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
	}
	assert.Check(t, is.DeepEqual(expected, volume))
}

func TestIsFilePath(t *testing.T) {
	assert.Check(t, !isFilePath("a界"))
}

// Preserve the test cases for VolumeSplitN
func TestParseVolumeSplitCases(t *testing.T) {
	for casenumber, x := range []struct {
		input    string
		n        int
		expected []string
	}{
		{`C:\foo:d:`, -1, []string{`C:\foo`, `d:`}},
		{`:C:\foo:d:`, -1, nil},
		{`/foo:/bar:ro`, 3, []string{`/foo`, `/bar`, `ro`}},
		{`/foo:/bar:ro`, 2, []string{`/foo`, `/bar:ro`}},
		{`C:\foo\:/foo`, -1, []string{`C:\foo\`, `/foo`}},
		{`d:\`, -1, []string{`d:\`}},
		{`d:`, -1, []string{`d:`}},
		{`d:\path`, -1, []string{`d:\path`}},
		{`d:\path with space`, -1, []string{`d:\path with space`}},
		{`d:\pathandmode:rw`, -1, []string{`d:\pathandmode`, `rw`}},

		{`c:\:d:\`, -1, []string{`c:\`, `d:\`}},
		{`c:\windows\:d:`, -1, []string{`c:\windows\`, `d:`}},
		{`c:\windows:d:\s p a c e`, -1, []string{`c:\windows`, `d:\s p a c e`}},
		{`c:\windows:d:\s p a c e:RW`, -1, []string{`c:\windows`, `d:\s p a c e`, `RW`}},
		{`c:\program files:d:\s p a c e i n h o s t d i r`, -1, []string{`c:\program files`, `d:\s p a c e i n h o s t d i r`}},
		{`0123456789name:d:`, -1, []string{`0123456789name`, `d:`}},
		{`MiXeDcAsEnAmE:d:`, -1, []string{`MiXeDcAsEnAmE`, `d:`}},
		{`name:D:`, -1, []string{`name`, `D:`}},
		{`name:D::rW`, -1, []string{`name`, `D:`, `rW`}},
		{`name:D::RW`, -1, []string{`name`, `D:`, `RW`}},

		{`c:/:d:/forward/slashes/are/good/too`, -1, []string{`c:/`, `d:/forward/slashes/are/good/too`}},
		{`c:\Windows`, -1, []string{`c:\Windows`}},
		{`c:\Program Files (x86)`, -1, []string{`c:\Program Files (x86)`}},
		{``, -1, nil},
		{`.`, -1, []string{`.`}},
		{`..\`, -1, []string{`..\`}},
		{`c:\:..\`, -1, []string{`c:\`, `..\`}},
		{`c:\:d:\:xyzzy`, -1, []string{`c:\`, `d:\`, `xyzzy`}},
		// Cover directories with one-character name
		{`/tmp/x/y:/foo/x/y`, -1, []string{`/tmp/x/y`, `/foo/x/y`}},
	} {
		parsed, _ := ParseVolume(x.input)

		expected := len(x.expected) > 1
		msg := fmt.Sprintf("Case %d: %s", casenumber, x.input)
		assert.Check(t, is.Equal(expected, parsed.Source != ""), msg)
	}
}

func TestParseVolumeInvalidEmptySpec(t *testing.T) {
	_, err := ParseVolume("")
	assert.ErrorContains(t, err, "invalid empty volume spec")
}

func TestParseVolumeInvalidSections(t *testing.T) {
	_, err := ParseVolume("/foo::rw")
	assert.ErrorContains(t, err, "invalid spec")
}

func TestVolumeStringer(t *testing.T) {
	v := types.ServiceVolumeConfig{
		Type:     "bind",
		Source:   "/src",
		Target:   "/target",
		ReadOnly: false,
		Bind: &types.ServiceVolumeBind{
			CreateHostPath: true,
			Propagation:    types.PropagationShared,
			SELinux:        types.SELinuxShared,
		},
	}
	assert.Equal(t, v.String(), "/src:/target:rw,z,shared")
}

// TestParseVolumeWithVariableDefaultValue tests that volume parsing
// correctly handles variables with default values (${VAR:-DEFAULT})
func TestParseVolumeWithVariableDefaultValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected types.ServiceVolumeConfig
	}{
		{
			name:  "variable with default value in target path",
			input: "/tmp:/tmp/${BUG_HERE:-DEFAULT}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${BUG_HERE:-DEFAULT}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with default value in source path",
			input: "/tmp/${VAR:-default}:/target",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp/${VAR:-default}",
				Target: "/target",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with default value in both source and target",
			input: "/src/${SRC:-default}:/dst/${DST:-value}",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/src/${SRC:-default}",
				Target: "/dst/${DST:-value}",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with empty default value",
			input: "/tmp:/tmp/${VAR:-}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${VAR:-}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with complex default value containing slashes",
			input: "/tmp:/tmp/${VAR:-default/path/value}/file",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${VAR:-default/path/value}/file",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "multiple variables with default values in target",
			input: "/tmp:/tmp/${VAR1:-val1}/${VAR2:-val2}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${VAR1:-val1}/${VAR2:-val2}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with default value and read-only option",
			input: "/tmp:/tmp/${VAR:-default}/path:ro",
			expected: types.ServiceVolumeConfig{
				Type:     "bind",
				Source:   "/tmp",
				Target:   "/tmp/${VAR:-default}/path",
				ReadOnly: true,
				Bind:     &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable without default value (normal variable)",
			input: "/tmp:/tmp/${WORKS_AS_EXPECTED}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${WORKS_AS_EXPECTED}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "mixed variables with and without defaults",
			input: "/tmp:/tmp/${VAR1:-default}/${VAR2}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${VAR1:-default}/${VAR2}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with default containing colon",
			input: "/tmp:/tmp/${TIME:-12:30:45}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${TIME:-12:30:45}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "named volume with variable default in target",
			input: "myvolume:/data/${VAR:-default}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "volume",
				Source: "myvolume",
				Target: "/data/${VAR:-default}/path",
				Volume: &types.ServiceVolumeVolume{},
			},
		},
		{
			name:  "variable with default value and bind options",
			input: "/src:/target/${VAR:-default}/path:slave,ro",
			expected: types.ServiceVolumeConfig{
				Type:     "bind",
				Source:   "/src",
				Target:   "/target/${VAR:-default}/path",
				ReadOnly: true,
				Bind: &types.ServiceVolumeBind{
					CreateHostPath: true,
					Propagation:    "slave",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			volume, err := ParseVolume(tc.input)
			assert.NilError(t, err, fmt.Sprintf("Failed to parse: %s", tc.input))
			assert.Check(t, is.DeepEqual(tc.expected, volume))
		})
	}
}

// TestParseVolumeWithVariableDefaultValueWindows tests Windows-specific
// volume parsing with variables containing default values
func TestParseVolumeWithVariableDefaultValueWindows(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected types.ServiceVolumeConfig
	}{
		{
			name:  "Windows path with variable default in target",
			input: "C:\\source:D:\\target\\${VAR:-default}\\path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "C:\\source",
				Target: "D:\\target\\${VAR:-default}\\path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "Windows path with variable default in source",
			input: "C:\\src\\${VAR:-default}:D:\\target",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "C:\\src\\${VAR:-default}",
				Target: "D:\\target",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "Windows path with variable default and options",
			input: "C:\\source:D:\\${VAR:-default}\\path:ro",
			expected: types.ServiceVolumeConfig{
				Type:     "bind",
				Source:   "C:\\source",
				Target:   "D:\\${VAR:-default}\\path",
				ReadOnly: true,
				Bind:     &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			volume, err := ParseVolume(tc.input)
			assert.NilError(t, err, fmt.Sprintf("Failed to parse: %s", tc.input))
			assert.Check(t, is.DeepEqual(tc.expected, volume))
		})
	}
}

// TestParseVolumeWithNestedBraces tests edge cases with nested or
// complex brace patterns
func TestParseVolumeWithNestedBraces(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected types.ServiceVolumeConfig
	}{
		{
			name:  "variable with default containing braces",
			input: "/tmp:/tmp/${VAR:-{default}}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/${VAR:-{default}}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with default at end of path",
			input: "/tmp:/tmp/path/${VAR:-default}",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/tmp/path/${VAR:-default}",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
		{
			name:  "variable with default at start of path",
			input: "/tmp:/${VAR:-default}/path",
			expected: types.ServiceVolumeConfig{
				Type:   "bind",
				Source: "/tmp",
				Target: "/${VAR:-default}/path",
				Bind:   &types.ServiceVolumeBind{CreateHostPath: true},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			volume, err := ParseVolume(tc.input)
			assert.NilError(t, err, fmt.Sprintf("Failed to parse: %s", tc.input))
			assert.Check(t, is.DeepEqual(tc.expected, volume))
		})
	}
}
