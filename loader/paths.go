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
	"strings"

	"github.com/compose-spec/compose-go/types"
)

// ResolveRelativePaths resolves relative paths based on project WorkingDirectory
func ResolveRelativePaths(project *types.Project) error {
	absComposeFiles, err := absComposeFiles(project.ComposeFiles)
	if err != nil {
		return err
	}
	project.ComposeFiles = absComposeFiles

	for i, s := range project.Services {
		ResolveServiceRelativePaths(project.WorkingDir, &s)
		project.Services[i] = s
	}

	for i, obj := range project.Configs {
		if obj.File != "" {
			obj.File = absPath(project.WorkingDir, obj.File)
			project.Configs[i] = obj
		}
	}

	for i, obj := range project.Secrets {
		if obj.File != "" {
			obj.File = resolveMaybeUnixPath(project.WorkingDir, obj.File)
			project.Secrets[i] = obj
		}
	}

	for name, config := range project.Volumes {
		if config.Driver == "local" && config.DriverOpts["o"] == "bind" {
			// This is actually a bind mount
			config.DriverOpts["device"] = resolveMaybeUnixPath(project.WorkingDir, config.DriverOpts["device"])
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
		localContext := absPath(workingDir, s.Build.Context)
		if _, err := os.Stat(localContext); err == nil {
			s.Build.Context = localContext
		}
		for name, path := range s.Build.AdditionalContexts {
			if strings.Contains(path, "://") { // `docker-image://` or any builder specific context type
				continue
			}
			if isRemoteContext(path) {
				continue
			}
			path = absPath(workingDir, path)
			if _, err := os.Stat(path); err == nil {
				s.Build.AdditionalContexts[name] = path
			}
		}
	}
	for j, f := range s.EnvFile {
		s.EnvFile[j] = absPath(workingDir, f)
	}

	if s.Extends != nil && s.Extends.File != "" {
		s.Extends.File = absPath(workingDir, s.Extends.File)
	}

	for i, vol := range s.Volumes {
		if vol.Type != types.VolumeTypeBind {
			continue
		}
		s.Volumes[i].Source = resolveMaybeUnixPath(workingDir, vol.Source)
	}
}

func absPath(workingDir string, filePath string) string {
	if strings.HasPrefix(filePath, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, filePath[1:])
	}
	if filepath.IsAbs(filePath) {
		return filePath
	}
	return filepath.Join(workingDir, filePath)
}

func absComposeFiles(composeFiles []string) ([]string, error) {
	for i, composeFile := range composeFiles {
		absComposefile, err := filepath.Abs(composeFile)
		if err != nil {
			return nil, err
		}
		composeFiles[i] = absComposefile
	}
	return composeFiles, nil
}

func isRemoteContext(maybeURL string) bool {
	for _, prefix := range []string{"https://", "http://", "git://", "github.com/", "git@"} {
		if strings.HasPrefix(maybeURL, prefix) {
			return true
		}
	}
	return false
}
