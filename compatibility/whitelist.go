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

package compatibility

import (
	"fmt"

	"github.com/compose-spec/compose-go/errdefs"
	"github.com/compose-spec/compose-go/types"
	"github.com/pkg/errors"
)

// WhiteList implement Checker interface by rejecting all attributes but those listed in a whitelist.
type WhiteList struct {
	Supported []string
	errors    []error
}

func (c *WhiteList) Errors() []error {
	return c.errors
}

func (c *WhiteList) Check(project *types.Project) {
	for i, service := range project.Services {
		c.CheckServiceConfig(&service)
		project.Services[i] = service
	}

	for i, network := range project.Networks {
		c.CheckNetworkConfig(&network)
		project.Networks[i] = network
	}

	for i, volume := range project.Volumes {
		c.CheckVolumeConfig(&volume)
		project.Volumes[i] = volume
	}

	for i, config := range project.Configs {
		c.CheckConfigsConfig(&config)
		project.Configs[i] = config
	}

	for i, secret := range project.Secrets {
		c.CheckSecretsConfig(&secret)
		project.Secrets[i] = secret
	}
}

func (c *WhiteList) supported(attribute string) bool {
	for _, s := range c.Supported {
		if s == attribute {
			return true
		}
	}
	return false
}

func (c *WhiteList) error(message string, args ...interface{}) {
	c.errors = append(c.errors, errors.Wrap(errdefs.ErrUnsupported, fmt.Sprintf(message, args...)))
}
