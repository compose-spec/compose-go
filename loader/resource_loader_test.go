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

// The local resource loader is idempotent: the loader hands the file Load
// returned to the next loader and to Dir, and none of them may apply the
// directory of the loader again, whatever way that directory is written.
func TestLocalResourceLoaderIsIdempotent(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	assert.NilError(t, os.MkdirAll(filepath.Join(root, "proj", "sub"), 0o700))
	assert.NilError(t, os.WriteFile(filepath.Join(root, "proj", "sub", "c.yml"), nil, 0o600))

	for name, dir := range map[string]string{
		"empty":    "",
		"dot":      ".",
		"relative": "proj",
		"absolute": filepath.Join(root, "proj"),
	} {
		t.Run(name, func(t *testing.T) {
			ref := filepath.Join("sub", "c.yml")
			if dir == "" {
				ref = filepath.Join("proj", ref)
			}
			if dir == "." {
				ref = filepath.Join("proj", ref)
			}
			l := newLocalResourceLoader(dir)
			assert.Assert(t, filepath.IsAbs(l.dir), "the directory of the loader is absolute: %q", l.dir)

			local, err := l.Load(t.Context(), ref)
			assert.NilError(t, err)
			assert.Equal(t, local, filepath.Join(root, "proj", "sub", "c.yml"))
			again, err := l.Load(t.Context(), local)
			assert.NilError(t, err)
			assert.Equal(t, again, local)
			assert.Equal(t, l.Dir(local), l.Dir(ref))
		})
	}
}

// Dir is relative to the directory of the loader when it can be, for the
// paths of the model to stay relative to the working directory of the project
func TestLocalResourceLoaderDir(t *testing.T) {
	root := t.TempDir()
	assert.NilError(t, os.MkdirAll(filepath.Join(root, "a", "b"), 0o700))
	assert.NilError(t, os.MkdirAll(filepath.Join(root, "c"), 0o700))
	l := newLocalResourceLoader(filepath.Join(root, "a"))

	tests := map[string]struct{ ref, dir string }{
		"file":             {"b/compose.yaml", "b"},
		"directory":        {"b", "b"},
		"file next to it":  {"compose.yaml", "."},
		"sibling project":  {"../c/compose.yaml", "../c"},
		"absolute file":    {filepath.Join(root, "c", "compose.yaml"), "../c"},
		"absolute project": {filepath.Join(root, "a", "b"), "b"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, l.Dir(tt.ref), filepath.FromSlash(tt.dir))
		})
	}
}
