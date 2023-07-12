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
	"strings"

	"github.com/compose-spec/compose-go/types"
)

// ResolveRelativePaths resolves relative paths based on the project directory
// and shell-style `~/foo` user-home relative paths.
func ResolveRelativePaths(project *types.Project) error {
	homeDir, _ := os.UserHomeDir()
	return ResolveRelativePathsWithHomeDir(project, homeDir)
}

// ResolveRelativePathsWithHomeDir resolves relative paths based on the project directory
// and shell-style `~/foo` user-home relative paths based on the homeDir.
func ResolveRelativePathsWithHomeDir(project *types.Project, homeDir string) error {
	ppr := ProjectPathResolver{
		ProjectDir: project.WorkingDir,
		HomeDir:    homeDir,
	}

	for i, s := range project.Services {
		ResolveServiceRelativePaths(ppr, &s)
		project.Services[i] = s
	}

	for i, obj := range project.Configs {
		if obj.File != "" {
			obj.File = ppr.Resolve(obj.File)
			project.Configs[i] = obj
		}
	}

	for i, obj := range project.Secrets {
		if obj.File != "" {
			obj.File = ppr.ResolveMaybeUnix(obj.File)
			project.Secrets[i] = obj
		}
	}

	for name, config := range project.Volumes {
		if config.Driver == "local" && config.DriverOpts["o"] == "bind" {
			// This is actually a bind mount
			config.DriverOpts["device"] = ppr.ResolveMaybeUnix(config.DriverOpts["device"])
			project.Volumes[name] = config
		}
	}
	return nil
}

func ResolveServiceRelativePaths(r ProjectPathResolver, s *types.ServiceConfig) {
	if s.Build != nil {
		if !isRemoteContext(s.Build.Context) {
			s.Build.Context = r.Resolve(s.Build.Context)
		}
		for name, path := range s.Build.AdditionalContexts {
			if strings.Contains(path, "://") { // `docker-image://` or any builder specific context type
				continue
			}
			if isRemoteContext(path) {
				continue
			}
			s.Build.AdditionalContexts[name] = r.Resolve(path)
		}
	}
	for i := range s.EnvFile {
		s.EnvFile[i] = r.Resolve(s.EnvFile[i])
	}

	if s.Extends != nil && s.Extends.File != "" {
		s.Extends.File = r.Resolve(s.Extends.File)
	}

	for i, vol := range s.Volumes {
		if vol.Type != types.VolumeTypeBind {
			continue
		}
		s.Volumes[i].Source = r.ResolveMaybeUnix(vol.Source)
	}
}

// isRemoteContext returns true if the value is a Git reference or HTTP(S) URL.
//
// Any other value is assumed to be a local filesystem path and returns false.
//
// See: https://github.com/moby/buildkit/blob/18fc875d9bfd6e065cd8211abc639434ba65aa56/frontend/dockerui/context.go#L76-L79
func isRemoteContext(maybeURL string) bool {
	for _, prefix := range []string{"https://", "http://", "git://", "ssh://", "github.com/", "git@"} {
		if strings.HasPrefix(maybeURL, prefix) {
			return true
		}
	}
	return false
}
