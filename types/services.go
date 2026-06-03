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

package types

import (
	"fmt"

	"go.yaml.in/yaml/v4"
)

// Services is a map of ServiceConfig
type Services map[string]ServiceConfig

// UnmarshalYAML decodes the services mapping and injects each map key into
// the corresponding ServiceConfig.Name field. Replaces the v2 nameServices
// mapstructure decode hook so the value populated on Project.Services is
// self-describing.
func (s *Services) UnmarshalYAML(value *yaml.Node) error {
	value = unwrapDocument(value)
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid services config type, expected mapping, got %v", value.Kind)
	}
	out := Services{}
	for i := 0; i+1 < len(value.Content); i += 2 {
		name := value.Content[i].Value
		var svc ServiceConfig
		if err := value.Content[i+1].Decode(&svc); err != nil {
			return fmt.Errorf("services.%s: %w", name, err)
		}
		svc.Name = name
		out[name] = svc
	}
	*s = out
	return nil
}

// GetProfiles retrieve the profiles implicitly enabled by explicitly targeting selected services
func (s Services) GetProfiles() []string {
	set := map[string]struct{}{}
	for _, service := range s {
		for _, p := range service.Profiles {
			set[p] = struct{}{}
		}
	}
	var profiles []string
	for k := range set {
		profiles = append(profiles, k)
	}
	return profiles
}

func (s Services) Filter(predicate func(ServiceConfig) bool) Services {
	services := Services{}
	for name, service := range s {
		if predicate(service) {
			services[name] = service
		}
	}
	return services
}
