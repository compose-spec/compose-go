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

	"github.com/compose-spec/compose-go/v2/format"
	"github.com/compose-spec/compose-go/v2/types"
)

// ConvertVolumesToMounts moves the volumes that the short syntax can't say from
// the `volumes` of the services, jobs and pre_start hooks of the project to their
// `mounts`, as the mounts they are equivalent to (see format.IsShortSyntaxExpressible
// and types.ServiceVolumeConfig.ToMount). The volumes the short syntax can say are
// left as they are, so that running it again changes nothing.
//
// The loader does it when Options.ConvertVolumesToMounts is set; it is exported
// for what adds volumes to a project after it was loaded.
func ConvertVolumesToMounts(project *types.Project) error {
	for _, services := range []types.Services{project.Services, project.DisabledServices} {
		for name, service := range services {
			where := fmt.Sprintf("service %q", name)
			if err := convertSpecVolumesToMounts(&service.ContainerSpec, where); err != nil {
				return err
			}
			for i := range service.PreStart {
				if err := convertSpecVolumesToMounts(&service.PreStart[i].ContainerSpec, fmt.Sprintf("%s pre_start[%d]", where, i)); err != nil {
					return err
				}
			}
			services[name] = service
		}
	}
	for _, jobs := range []types.Jobs{project.Jobs, project.DisabledJobs} {
		for name, job := range jobs {
			where := fmt.Sprintf("job %q", name)
			if err := convertSpecVolumesToMounts(&job.ContainerSpec, where); err != nil {
				return err
			}
			jobs[name] = job
		}
	}
	return nil
}

func convertSpecVolumesToMounts(spec *types.ContainerSpec, where string) error {
	var volumes []types.ServiceVolumeConfig
	var mounts []types.ServiceMountConfig
	for _, volume := range spec.Volumes {
		if format.IsShortSyntaxExpressible(volume) {
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
	spec.Mounts = append(spec.Mounts, mounts...)
	return nil
}
