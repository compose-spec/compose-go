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
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestExpandPath(t *testing.T) {
	workingDir := "/foo"
	home, err := os.UserHomeDir()
	assert.NilError(t, err)
	/*
		u, err := user.Current()
		assert.NilError(t, err)
	*/

	type args struct {
		path     string
		expected string
	}
	tests := []args{
		{
			path:     "",
			expected: "",
		},
		{
			path:     "/tmp/foo",
			expected: "/tmp/foo",
		},
		{
			path:     "./bar",
			expected: abs("/foo/bar"),
		},
		{
			path:     "../bar",
			expected: abs("/bar"),
		},
		{
			path:     "~/bar",
			expected: filepath.Join(home, "bar"),
		},
		{
			path:     "~",
			expected: home,
		},
		/*
			{
				path:     "~" + u.Username,
				expected: u.HomeDir,
			},
			{
				path:     "~" + u.Username + "/foo",
				expected: filepath.Join(u.HomeDir, "foo"),
			},*/
		{
			path:     "c:/foo",
			expected: "c:/foo",
		},
		{
			path:     "\\\\server\\share\\path\\file",
			expected: "\\\\server\\share\\path\\file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, err := ExpandPath(workingDir, tt.path)
			assert.NilError(t, err)
			assert.Equal(t, got, tt.expected)
		})
	}
}

func abs(s string) string {
	abs, _ := filepath.Abs(s)
	return abs
}
