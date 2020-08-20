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
	"os/user"
	"path/filepath"
	"strings"
)

// ExpandPath expands "~", "~user" and "." in paths to produce absolute paths
func ExpandPath(workingDir string, path string) (string, error) {
	if len(path) == 0 {
		return path, nil
	}

	// Detect windows/unix absolute path, typically used as volume source
	if strings.HasPrefix(path, "/") || isAbs(path) {
		return path, nil
	}

	if strings.HasPrefix(path, "~") {
		i := 1
		for ; i < len(path); i++ {
			if path[i] == '/' || path[i] == '\\' {
				break
			}
		}
		usr := path[1:i]
		var home string
		if usr == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return path, err
			}
			home = homeDir
		} else {
			usr, err := user.Lookup(usr)
			if err != nil {
				return "", err
			}
			home = usr.HomeDir
		}
		path = strings.Replace(path, "~"+usr, home, 1)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDir, path)
	}
	return filepath.Abs(path)
}
