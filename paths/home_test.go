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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

// setHome points the home directory lookup of os.UserHomeDir at dir on every
// OS ("HOME" on Unix, "USERPROFILE" on Windows).
func setHome(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
}

// A leading "~" is the user's home directory: the spec lets bind mount
// sources, env files... be written "~/something", and the resolved value must
// be rooted in $HOME so the engine never sees a literal "~".
// Anything that does not *start* with "~", or where "~" is followed by a name
// (the home directory of another user, which is not looked up), is left
// untouched.
func TestExpandUser(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "home itself", input: "~", want: home},
		{name: "path below home", input: "~/data/db", want: filepath.Join(home, "data", "db")},
		{name: "trailing slash is cleaned", input: "~/data/", want: filepath.Join(home, "data")},
		{name: "absolute path is untouched", input: "/srv/data", want: "/srv/data"},
		{name: "relative path is untouched", input: "./data", want: "./data"},
		{name: "tilde in the middle is untouched", input: "data/~/db", want: "data/~/db"},
		{name: "empty string is untouched", input: "", want: ""},
		{name: "another user's home is untouched", input: "~user/data", want: "~user/data"},
		{name: "another user's home alone is untouched", input: "~user", want: "~user"},
		{name: "tilde followed by a tilde is untouched", input: "~~/data", want: "~~/data"},
		{name: "path below home with the OS separator", input: "~" + string(filepath.Separator) + "data", want: filepath.Join(home, "data")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, ExpandUser(tt.input), tt.want)
		})
	}
}

// When the home directory cannot be determined, "~" cannot be expanded: the
// value must be returned as is (not turned into a bogus path) and the user
// told why through a warning, instead of failing the whole load.
func TestExpandUserWithoutHome(t *testing.T) {
	setHome(t, "")

	var logs bytes.Buffer
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(os.Stderr) })

	assert.Equal(t, ExpandUser("~/data"), "~/data")
	assert.Assert(t, bytes.Contains(logs.Bytes(), []byte("cannot expand '~'")), logs.String())
}
