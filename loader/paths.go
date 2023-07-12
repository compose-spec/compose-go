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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/compose-spec/compose-go/types"
)

// ResolveRelativePaths resolves relative paths based on project WorkingDirectory
func ResolveRelativePaths(project *types.Project) error {
	projDir, err := RealAbsPath("", project.WorkingDir)
	if err != nil {
		return err
	}
	project.WorkingDir = projDir

	for i := range project.ComposeFiles {
		// N.B. Relative Compose files paths are resolved based on the process
		// current directory
		p, err := RealAbsPath("", project.ComposeFiles[i])
		if err != nil {
			return err
		}
		project.ComposeFiles[i] = p
	}

	for i, s := range project.Services {
		ResolveServiceRelativePaths(project.WorkingDir, &s)
		project.Services[i] = s
	}

	for i, obj := range project.Configs {
		if obj.File != "" {
			p, err := RealAbsPath(project.WorkingDir, obj.File)
			if err != nil {
				return err
			}
			obj.File = p
			project.Configs[i] = obj
		}
	}

	for i, obj := range project.Secrets {
		if obj.File != "" {
			p, err := RealAbsPath(project.WorkingDir, obj.File)
			if err != nil {
				return err
			}
			obj.File = p
			project.Secrets[i] = obj
		}
	}

	for name, config := range project.Volumes {
		if config.Driver == "local" && config.DriverOpts["o"] == "bind" {
			p, err := RealAbsPath(project.WorkingDir, config.DriverOpts["device"])
			if err != nil {
				return err
			}
			// This is actually a bind mount
			config.DriverOpts["device"] = p
			project.Volumes[name] = config
		}
	}
	return nil
}

func ResolveServiceRelativePaths(workingDir string, s *types.ServiceConfig) {
	if s.Build != nil && s.Build.Context != "" && !isRemoteContext(s.Build.Context) {
		// Build context might be a remote http/git context. Unfortunately supported "remote"
		// syntax is highly ambiguous in moby/moby and not defined by compose-spec,
		// so let's assume runtime will check
		if localContext, err := RealAbsPath(workingDir, s.Build.Context); err == nil {
			if _, err := os.Stat(localContext); err == nil {
				s.Build.Context = localContext
			}
		}
		for name, path := range s.Build.AdditionalContexts {
			if strings.Contains(path, "://") { // `docker-image://` or any builder specific context type
				continue
			}
			if isRemoteContext(path) {
				continue
			}
			if path, err := RealAbsPath(workingDir, path); err == nil {
				if _, err := os.Stat(path); err == nil {
					s.Build.AdditionalContexts[name] = path
				}
			}
		}
	}
	for j, f := range s.EnvFile {
		p, err := RealAbsPath(workingDir, f)
		if err == nil {
			s.EnvFile[j] = p
		}
	}

	if s.Extends != nil && s.Extends.File != "" {
		p, err := RealAbsPath(workingDir, s.Extends.File)
		if err == nil {
			s.Extends.File = p
		}
	}

	for i, vol := range s.Volumes {
		if vol.Type != types.VolumeTypeBind {
			continue
		}
		p, err := resolveMaybeUnixPath(workingDir, vol.Source)
		if err == nil {
			s.Volumes[i].Source = p
		}
	}
}

// RealAbsPath attempts to resolve symlinks and determine the absolute, canonical path.
//
// Requires that the path exists or an error will be returned.
func RealAbsPath(basePath string, path string) (string, error) {
	path, err := expandUser(path)
	if err != nil {
		return "", err
	}

	if pathRelToHome, ok := strings.CutPrefix(path, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, pathRelToHome)
	}

	var absPath string
	switch {
	case filepath.IsAbs(path):
		absPath = filepath.Clean(path)
	case basePath != "":
		absPath = filepath.Join(basePath, path)
	default:
		var err error
		absPath, err = filepath.Abs(path)
		if err != nil {
			return "", err
		}
	}

	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return absPath, nil
		}
		return "", err
	}
	return realPath, nil
}

func isRemoteContext(maybeURL string) bool {
	for _, prefix := range []string{"https://", "http://", "git://", "github.com/", "git@"} {
		if strings.HasPrefix(maybeURL, prefix) {
			return true
		}
	}
	return false
}

func expandUser(path string) (string, error) {
	if pathRelToHome, ok := strings.CutPrefix(path, "~"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand '~': %v", err)
		}
		path = filepath.Join(home, pathRelToHome)
	}
	return path, nil
}
