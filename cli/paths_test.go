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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func TestRealAbsPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Test creates real files on disk")
	}

	tmpdir := temporaryWorkDir(t)

	realDir := filepath.Join(tmpdir, "real")
	assert.Equal(t, true, filepath.IsAbs(realDir))
	assert.NilError(t, os.Mkdir(realDir, 0o700))
	linkDir := filepath.Join(tmpdir, "symlink")
	assert.NilError(t, os.Symlink(realDir, linkDir))

	var p string
	var err error

	// reflexive
	p, err = RealAbsPath(realDir)
	assert.NilError(t, err)
	assert.Equal(t, realDir, p)

	// follow symlink
	p, err = RealAbsPath(linkDir)
	assert.NilError(t, err)
	assert.Equal(t, realDir, p)

	// reflexive (relative)
	p, err = RealAbsPath("./real")
	assert.NilError(t, err)
	assert.Equal(t, realDir, p)

	// follow symlink (relative)
	p, err = RealAbsPath("./symlink")
	assert.NilError(t, err)
	assert.Equal(t, realDir, p)

	// non-existent
	p, err = RealAbsPath("./does-not-exist")
	assert.NilError(t, err)
	assert.Equal(t, filepath.Join(tmpdir, "does-not-exist"), p)
}

func temporaryWorkDir(t testing.TB) string {
	t.Helper()

	// the tmpdir might itself be a symlink (e.g. on macOS)
	tmpdir, err := filepath.EvalSymlinks(t.TempDir())
	assert.NilError(t, err)

	wd, err := os.Getwd()
	assert.NilError(t, err)
	// NOTE: cleanup funcs are executed LIFO, so we need to restore
	// the old dir before the `t.TempDir()` cleanup fires or it'll
	// fail on Windows since the path will still be in use (by us)
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			panic(fmt.Errorf("restoring working directory: %v", err))
		}
	})

	assert.NilError(t, os.Chdir(tmpdir))
	return tmpdir
}
