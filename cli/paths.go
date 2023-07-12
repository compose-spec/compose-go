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
	"io/fs"
	"path/filepath"

	"github.com/pkg/errors"
)

// RealAbsPath attempts to determine the true absolute path after symlink evaluation.
func RealAbsPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// symlink resolution is done to ensure we use the canonical
			// version of a path; if it does not exist, return the absolute
			// path, allowing a failure to occur at the natural point when
			// the path is attempted to be read.
			return path, nil
		}
		return "", err
	}
	return filepath.Abs(realPath)
}
