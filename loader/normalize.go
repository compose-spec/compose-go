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
	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// normalize compose project by moving deprecated attributes to their canonical position and injecting implicit defaults
func normalize(project *types.Project) error {
	// If none defined, Compose model involves an implicit "default" network
	if len(project.Networks) == 0 {
		project.Networks["default"] = types.NetworkConfig{}
	}

	for i, s := range project.Services {
		if len(s.Networks) == 0 {
			// Service without explicit network attachment are implicitly exposed on default network
			s.Networks = map[string]*types.ServiceNetworkConfig{"default": nil}
		}

		err := relocateLogDriver(s)
		if err != nil {
			return err
		}

		err = relocateLogOpt(s)
		if err != nil {
			return err
		}

		err = relocateDockerfile(s)
		if err != nil {
			return err
		}

		project.Services[i] = s
	}
	return nil
}

func relocateLogOpt(s types.ServiceConfig) error {
	if len(s.LogOpt) != 0 {
		logrus.Warn("`log_opts` is deprecated. Use the `logging` element")
		if s.Logging == nil {
			s.Logging = &types.LoggingConfig{}
		}
		for k, v := range s.LogOpt {
			if _, ok := s.Logging.Options[k]; !ok {
				s.Logging.Options[k] = v
			} else {
				return errors.Wrap(errdefs.ErrInvalid, "can't use both 'log_opt' (deprecated) and 'logging.options'")
			}
		}
	}
	return nil
}

func relocateLogDriver(s types.ServiceConfig) error {
	if s.LogDriver != "" {
		logrus.Warn("`log_driver` is deprecated. Use the `logging` element")
		if s.Logging == nil {
			s.Logging = &types.LoggingConfig{}
		}
		if s.Logging.Driver == "" {
			s.Logging.Driver = s.LogDriver
		} else {
			return errors.Wrap(errdefs.ErrInvalid, "can't use both 'log_driver' (deprecated) and 'logging.driver'")
		}
	}
	return nil
}

func relocateDockerfile(s types.ServiceConfig) error {
	if s.Dockerfile != "" {
		logrus.Warn("`dockerfile` is deprecated. Use the `build` element")
		if s.Build == nil {
			s.Build = &types.BuildConfig{}
		}
		if s.Dockerfile == "" {
			s.Build.Dockerfile = s.Dockerfile
		} else {
			return errors.Wrap(errdefs.ErrInvalid, "can't use both 'dockerfile' (deprecated) and 'build.dockerfile'")
		}
	}
	return nil
}
