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
	paths "path"
	"path/filepath"
	"strings"
)

// ProjectPathResolver resolves file path references within a Compose file to suitable
// paths for use.
//
// Relative paths are joined to the ProjectDir (note: this might still be a relative
// path if ProjectDir is itself relative).
//
// Shell-style user-home relative paths such as `~/foo` are joined to the HomeDir
// if specified. If HomeDir is empty, they will be left as-is.
//
// All returned paths are cleaned for consistency.
type ProjectPathResolver struct {
	// ProjectDir is the path to the project working directory.
	//
	// Any relative paths in a Compose file are based off this.
	ProjectDir string

	// HomeDir is the user's home directory path.
	//
	// If non-empty, any shell-style user-home relative paths (e.g. `~/foo`)
	// are based off this.
	//
	// If empty, any shell-style user-home relative paths are returned as-is.
	HomeDir string
}

// Resolve checks if the value is an absolute path for the OS loading the project.
func (r ProjectPathResolver) Resolve(path string) string {
	if strings.HasPrefix(path, "~") {
		if r.HomeDir == "" {
			// this is a user-home relative path no
			// homedir was specified, so leave it as-is
			return filepath.Clean(path)
		}
		path = filepath.Join(r.HomeDir, path[1:])
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(r.ProjectDir, path)
	}
	path = filepath.Clean(path)
	return path
}

// ResolveMaybeUnix checks if the value is an absolute path (either Unix or Windows) to
// handle a Windows client with a Unix daemon or vice-versa.
//
// Note that this is not required for Docker for Windows when specifying
// a local Windows path, because Docker for Windows translates the Windows
// path into a valid path within the VM.
func (r ProjectPathResolver) ResolveMaybeUnix(path string) string {
	if !paths.IsAbs(path) && !isAbs(path) {
		return r.Resolve(path)
	}
	return path
}
