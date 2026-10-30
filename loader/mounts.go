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
	"maps"
	"slices"

	"github.com/compose-spec/compose-go/v2/types"
)

// ConvertVolumesToMounts moves the volumes for which convert returns true from the
// `volumes` of the services (disabled ones too), jobs (idem) and pre_start hooks of the
// project to their `mounts`, as the mounts they are equivalent to (see
// types.ServiceVolumeConfig.ToMount, which fails for a volume that sets what a mount can't
// carry). The other volumes are left as they are, so that running it again changes nothing,
// as long as convert only depends on the volume it is given, without state: it sees the
// volumes as the project has them, defaults of the normalization included. Nothing is moved
// when convert is nil.
//
// An error of convert, or of ToMount, is returned with the container and the volume it
// is about, and leaves the project as it was: all is converted or nothing is, the
// containers being visited in the order of their names.
//
// The loader does it when Options.ConvertVolumesToMounts is set; it is exported
// for what adds volumes to a project after it was loaded.
func ConvertVolumesToMounts(project *types.Project, convert func(types.ServiceVolumeConfig) (bool, error)) error {
	if convert == nil {
		return nil
	}
	// the converted containers are kept apart until all of them are, so that an error
	// leaves the project untouched
	serviceSets := []types.Services{project.Services, project.DisabledServices}
	converted := make([]types.Services, len(serviceSets))
	for i, services := range serviceSets {
		converted[i] = types.Services{}
		for _, name := range slices.Sorted(maps.Keys(services)) {
			service := services[name]
			where := fmt.Sprintf("service %q", name)
			if err := convertSpecVolumesToMounts(&service.ContainerSpec, where, convert); err != nil {
				return err
			}
			// the hooks are copied, so that what is converted is not what the project holds
			service.PreStart = slices.Clone(service.PreStart)
			for j := range service.PreStart {
				if err := convertSpecVolumesToMounts(&service.PreStart[j].ContainerSpec, fmt.Sprintf("%s pre_start[%d]", where, j), convert); err != nil {
					return err
				}
			}
			converted[i][name] = service
		}
	}
	jobSets := []types.Jobs{project.Jobs, project.DisabledJobs}
	convertedJobs := make([]types.Jobs, len(jobSets))
	for i, jobs := range jobSets {
		convertedJobs[i] = types.Jobs{}
		for _, name := range slices.Sorted(maps.Keys(jobs)) {
			job := jobs[name]
			if err := convertSpecVolumesToMounts(&job.ContainerSpec, fmt.Sprintf("job %q", name), convert); err != nil {
				return err
			}
			convertedJobs[i][name] = job
		}
	}
	for i, services := range serviceSets {
		maps.Copy(services, converted[i])
	}
	for i, jobs := range jobSets {
		maps.Copy(jobs, convertedJobs[i])
	}
	return nil
}

// convertSpecVolumesToMounts converts the volumes of spec, which it leaves as it was
// when it fails: it sets the slices it changes rather than writing in the ones it was given.
func convertSpecVolumesToMounts(spec *types.ContainerSpec, where string, convert func(types.ServiceVolumeConfig) (bool, error)) error {
	var volumes []types.ServiceVolumeConfig
	var mounts []types.ServiceMountConfig
	for _, volume := range spec.Volumes {
		toMount, err := convert(volume)
		if err != nil {
			return fmt.Errorf("%s: volume %s: %w", where, volume.Target, err)
		}
		if !toMount {
			volumes = append(volumes, volume)
			continue
		}
		mount, err := volume.ToMount()
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		mounts = append(mounts, mount)
	}
	if len(mounts) == 0 {
		return nil
	}
	spec.Volumes = volumes
	spec.Mounts = append(slices.Clone(spec.Mounts), mounts...)
	return nil
}
