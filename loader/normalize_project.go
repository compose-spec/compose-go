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
	"path"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/paths"
	"github.com/compose-spec/compose-go/v2/types"
)

// normalizeProject applies post-decode normalization on a typed Project.
// This replaces the old Normalize() that operated on map[string]any.
func normalizeProject(project *types.Project) {
	// 1. Set service names from map keys
	for name, svc := range project.Services {
		svc.Name = name
		project.Services[name] = svc
	}

	// 2. Normalize networks (inject implicit default network)
	normalizeProjectNetworks(project)

	// 3. Set resource names from map keys with project name prefix
	setResourceNames(project)

	for name, svc := range project.Services {
		// 4. Normalize pull_policy
		if svc.PullPolicy == types.PullPolicyIfNotPresent {
			svc.PullPolicy = types.PullPolicyMissing
		}

		// 5. Build defaults
		if svc.Build != nil {
			if svc.Build.Context == "" {
				svc.Build.Context = "."
			}
			if svc.Build.Dockerfile == "" && svc.Build.DockerfileInline == "" {
				svc.Build.Dockerfile = "Dockerfile"
			}
		}

		// 6. Infer depends_on from links, network_mode, volumes_from
		inferDependsOn(&svc)

		// 7. Clean volume paths
		for i, vol := range svc.Volumes {
			svc.Volumes[i].Target = path.Clean(vol.Target)
			if vol.Source != "" {
				if path.IsAbs(vol.Source) {
					// Preserve Unix-style absolute paths (e.g. /opt/data)
					// which filepath.Clean would convert to \opt\data on Windows
					svc.Volumes[i].Source = path.Clean(vol.Source)
				} else {
					svc.Volumes[i].Source = filepath.Clean(vol.Source)
				}
			}
		}

		// 8. Default values for ports
		for i, p := range svc.Ports {
			if p.Protocol == "" {
				svc.Ports[i].Protocol = "tcp"
			}
			if p.Mode == "" {
				svc.Ports[i].Mode = "ingress"
			}
		}

		// 9. Default values for secrets/configs mounts
		for i, s := range svc.Secrets {
			if s.Target == "" {
				svc.Secrets[i].Target = fmt.Sprintf("/run/secrets/%s", s.Source)
			}
		}

		// 10. Default values for volume bind
		for i, vol := range svc.Volumes {
			if vol.Type == types.VolumeTypeBind && vol.Bind != nil && !vol.Bind.CreateHostPath {
				// The old pipeline sets create_host_path=true by default.
				// In the new pipeline, the zero value is false. We set it to true
				// only when Bind section exists but create_host_path was not explicitly set.
				svc.Volumes[i].Bind.CreateHostPath = true
			}
		}

		// 11. Default values for device requests
		if svc.Deploy != nil && svc.Deploy.Resources.Reservations != nil {
			for i, dev := range svc.Deploy.Resources.Reservations.Devices {
				if dev.Count == 0 && len(dev.IDs) == 0 {
					all := types.DeviceCount(-1) // "all"
					svc.Deploy.Resources.Reservations.Devices[i].Count = all
				}
			}
		}
		for i, gpu := range svc.Gpus {
			if gpu.Count == 0 && len(gpu.IDs) == 0 {
				all := types.DeviceCount(-1)
				svc.Gpus[i].Count = all
			}
		}

		project.Services[name] = svc
	}

	// 12. Resolve secrets/configs environment references
	resolveSecretConfigEnvironment(project)
}

func normalizeProjectNetworks(project *types.Project) {
	usesDefaultNetwork := false

	for name, svc := range project.Services {
		if svc.Provider != nil {
			continue
		}
		if svc.NetworkMode != "" {
			continue
		}
		if len(svc.Networks) == 0 {
			svc.Networks = types.ServiceNetworks{
				"default": nil,
			}
			usesDefaultNetwork = true
		} else if _, ok := svc.Networks["default"]; ok {
			usesDefaultNetwork = true
		}
		project.Services[name] = svc
	}

	if usesDefaultNetwork {
		if project.Networks == nil {
			project.Networks = types.Networks{}
		}
		if _, ok := project.Networks["default"]; !ok {
			project.Networks["default"] = types.NetworkConfig{}
		}
	}
}

func setResourceNames(project *types.Project) {
	setNames := func(name string, externalName string, external types.External, projectName string) string {
		if name != "" {
			return name
		}
		if bool(external) {
			return externalName
		}
		return fmt.Sprintf("%s_%s", projectName, externalName)
	}

	for key, net := range project.Networks {
		net.Name = setNames(net.Name, key, net.External, project.Name)
		project.Networks[key] = net
	}
	for key, vol := range project.Volumes {
		vol.Name = setNames(vol.Name, key, vol.External, project.Name)
		project.Volumes[key] = vol
	}
	for key, cfg := range project.Configs {
		cfg.Name = setNames(cfg.Name, key, cfg.External, project.Name)
		project.Configs[key] = cfg
	}
	for key, sec := range project.Secrets {
		sec.Name = setNames(sec.Name, key, sec.External, project.Name)
		project.Secrets[key] = sec
	}
}

func inferDependsOn(svc *types.ServiceConfig) {
	if svc.DependsOn == nil {
		svc.DependsOn = types.DependsOnConfig{}
	}

	addDep := func(name string, restart bool) {
		if _, ok := svc.DependsOn[name]; !ok {
			svc.DependsOn[name] = types.ServiceDependency{
				Condition: types.ServiceConditionStarted,
				Restart:   restart,
				Required:  true,
			}
		}
	}

	// From links
	for _, link := range svc.Links {
		parts := strings.Split(link, ":")
		addDep(parts[0], true)
	}

	// From namespace references (network_mode, ipc, pid, uts, cgroup)
	for _, ref := range []string{svc.NetworkMode, svc.Ipc, svc.Pid, svc.Uts, svc.Cgroup} {
		if strings.HasPrefix(ref, types.ServicePrefix) {
			addDep(ref[len(types.ServicePrefix):], true)
		}
	}

	// From volumes_from
	for _, vol := range svc.VolumesFrom {
		if !strings.HasPrefix(vol, types.ContainerPrefix) {
			spec := strings.Split(vol, ":")
			addDep(spec[0], false)
		}
	}

	// Remove empty depends_on to match old behavior
	if len(svc.DependsOn) == 0 {
		svc.DependsOn = nil
	}
}

func resolveSecretConfigEnvironment(project *types.Project) {
	for name, secret := range project.Secrets {
		if secret.Environment != "" {
			if val, ok := project.Environment[secret.Environment]; ok {
				secret.Content = val
			}
			project.Secrets[name] = secret
		}
	}
	for name, config := range project.Configs {
		if config.Environment != "" {
			if val, ok := project.Environment[config.Environment]; ok {
				config.Content = val
			}
			project.Configs[name] = config
		}
	}
}

// isRemoteContext checks if a build context value is a remote reference (Git, HTTP, etc.)
func isRemoteContext(v string) bool {
	for _, prefix := range []string{"https://", "http://", "git://", "ssh://", "github.com/", "git@"} {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

// resolveProjectPaths resolves relative paths in a typed Project.
// This replaces the old paths.ResolveRelativePaths that operated on map[string]any.
func resolveProjectPaths(project *types.Project, opts *Options) error { //nolint:gocyclo
	workDir := project.WorkingDir

	var remoteCheck []paths.RemoteResource
	for _, loader := range opts.RemoteResourceLoaders() {
		remoteCheck = append(remoteCheck, loader.Accept)
	}
	isRemote := func(p string) bool {
		for _, check := range remoteCheck {
			if check(p) {
				return true
			}
		}
		return false
	}

	absPath := func(p string) string {
		p = paths.ExpandUser(p)
		if filepath.IsAbs(p) || path.IsAbs(p) || p == "" {
			return p
		}
		return filepath.Join(workDir, p)
	}

	for name, svc := range project.Services {
		// Build context
		if svc.Build != nil {
			ctx := svc.Build.Context
			if ctx != "" && !strings.Contains(ctx, "://") &&
				!strings.HasPrefix(ctx, types.ServicePrefix) && !isRemote(ctx) {
				svc.Build.Context = absPath(ctx)
			}
			for k, v := range svc.Build.AdditionalContexts {
				if !strings.Contains(v, "://") && !isRemote(v) && !isRemoteContext(v) {
					svc.Build.AdditionalContexts[k] = absPath(v)
				}
			}
			for i, key := range svc.Build.SSH {
				if key.Path != "" {
					svc.Build.SSH[i].Path = absPath(key.Path)
				}
			}
		}

		// Env files
		for i, ef := range svc.EnvFiles {
			svc.EnvFiles[i].Path = absPath(ef.Path)
		}

		// Label files
		for i, lf := range svc.LabelFiles {
			svc.LabelFiles[i] = absPath(lf)
		}

		// Volumes (bind mounts)
		for i, vol := range svc.Volumes {
			if vol.Type == types.VolumeTypeBind {
				if vol.Source == "" {
					return fmt.Errorf(`invalid mount config for type "bind": field Source must not be empty`)
				}
				src := paths.ExpandUser(vol.Source)
				if !filepath.IsAbs(src) && !path.IsAbs(src) && !paths.IsWindowsAbs(src) {
					svc.Volumes[i].Source = filepath.Join(workDir, src)
				} else {
					svc.Volumes[i].Source = src
				}
			}
		}

		// Extends file
		if svc.Extends != nil && svc.Extends.File != "" && !isRemote(svc.Extends.File) {
			svc.Extends.File = absPath(svc.Extends.File)
		}

		// Develop watch paths
		if svc.Develop != nil {
			for i, w := range svc.Develop.Watch {
				svc.Develop.Watch[i].Path = absPath(w.Path)
			}
		}

		project.Services[name] = svc
	}

	// Configs
	for name, cfg := range project.Configs {
		if cfg.File != "" {
			cfg.File = absPath(cfg.File)
			project.Configs[name] = cfg
		}
	}

	// Secrets
	for name, sec := range project.Secrets {
		if sec.File != "" {
			sec.File = absPath(sec.File)
			project.Secrets[name] = sec
		}
	}

	// Volumes with local driver + bind
	for name, vol := range project.Volumes {
		if vol.Driver == "local" && vol.DriverOpts != nil {
			if dev, ok := vol.DriverOpts["device"]; ok && vol.DriverOpts["o"] == "bind" {
				vol.DriverOpts["device"] = absPath(dev)
				project.Volumes[name] = vol
			}
		}
	}

	return nil
}
