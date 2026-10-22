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

package paths

import (
	"path/filepath"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
)

// Windows path flavors that must be recognized on any OS, so that a Windows
// client (or a project written for one) is not mistaken about what is an
// absolute path, and a relative path is never glued to a Windows-only prefix.
// Each case is the volume prefix Windows itself reports for the path: drive
// letter, UNC share, and the \\.\ and \\?\ device namespaces.
// https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats
var volumeNameCases = []struct {
	name   string
	path   string
	volume string
	abs    bool
}{
	// no volume
	{"empty", "", "", false},
	{"relative", `foo\bar`, "", false},
	{"rooted backslash", `\foo`, "", false},
	{"rooted slash", "/foo", "", false},

	// drive letters
	{"drive only", "C:", "C:", false},
	{"drive relative", `C:foo`, "C:", false},
	{"drive absolute", `C:\foo`, "C:", true},
	{"drive absolute slash", "c:/foo", "c:", true},

	// UNC
	{"unc share", `\\host\share\dir`, `\\host\share`, true},
	{"unc share slash", "//host/share/dir", "//host/share", true},
	{"unc share only", `\\host\share`, `\\host\share`, true},
	{"unc host only", `\\host`, `\\host`, true},
	{"unc empty", `\\`, `\\`, true},
	{"unc dot is not a device", `\\.x\foo`, `\\.x\foo`, true},

	// Win32 local device namespace
	{"device dot", `\\.`, `\\.`, true},
	{"device dot with trailing slash", `\\.\`, `\\.\`, true},
	{"device drive", `\\.\C:\foo`, `\\.\C:`, true},
	{"device name without path", `\\.\C:`, `\\.\C:`, true},
	{"device longer name", `\\.\UNCx\a`, `\\.\UNCx`, true},

	// Win32 file namespace and NT namespace
	{"file namespace", `\\?\C:\foo`, `\\?\C:`, true},
	{"file namespace question mark only", `\\?`, `\\?`, true},
	{"nt namespace", `\??\C:\foo`, `\??\C:`, true},

	// UNC through the device namespace, matched case-insensitively
	{"device unc", `\\.\UNC\host\share\foo`, `\\.\UNC\host\share`, true},
	{"device unc lower case", `\\.\unc\host\share\foo`, `\\.\unc\host\share`, true},
	{"device unc without share", `\\.\UNC`, `\\.\UNC`, true},
}

// volumeNameLen is what IsWindowsAbs is built on: it must measure the same
// volume prefix as Windows does for every flavor of path.
func TestVolumeNameLen(t *testing.T) {
	for _, tt := range volumeNameCases {
		t.Run(tt.name, func(t *testing.T) {
			got := volumeNameLen(tt.path)
			assert.Equal(t, tt.path[:got], tt.volume)
		})
	}
}

// IsWindowsAbs must agree on the flavors above, whatever the OS it runs on.
func TestIsWindowsAbsFlavors(t *testing.T) {
	for _, tt := range volumeNameCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, IsWindowsAbs(tt.path), tt.abs, tt.path)
		})
	}
}

// IsWindowsAbs and volumeNameLen are a copy of the Go standard library: on
// Windows, where the standard library is the reference, they must keep giving
// its answers so that the copy cannot drift from the real thing.
func TestWindowsPathsMatchStandardLibrary(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the standard library only implements Windows paths on Windows")
	}
	for _, tt := range volumeNameCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, volumeNameLen(tt.path), len(filepath.VolumeName(tt.path)))
			assert.Equal(t, IsWindowsAbs(tt.path), filepath.IsAbs(tt.path))
		})
	}
}
