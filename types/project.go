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
	"os"
	"path/filepath"
	"sort"
)

// Project is the result of loading a set of compose files
type Project struct {
	Name       string
	WorkingDir string
	Services   Services               `json:"services"`
	Networks   Networks               `yaml:",omitempty" json:"networks,omitempty"`
	Volumes    Volumes                `yaml:",omitempty" json:"volumes,omitempty"`
	Secrets    Secrets                `yaml:",omitempty" json:"secrets,omitempty"`
	Configs    Configs                `yaml:",omitempty" json:"configs,omitempty"`
	Extensions map[string]interface{} `yaml:",inline" json:"-"`
}

// ServiceNames return names for all services in this Compose config
func (p Project) ServiceNames() []string {
	names := []string{}
	for _, s := range p.Services {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	return names
}

// VolumeNames return names for all volumes in this Compose config
func (p Project) VolumeNames() []string {
	names := []string{}
	for k := range p.Volumes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// NetworkNames return names for all volumes in this Compose config
func (p Project) NetworkNames() []string {
	names := []string{}
	for k := range p.Networks {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// SecretNames return names for all secrets in this Compose config
func (p Project) SecretNames() []string {
	names := []string{}
	for k := range p.Secrets {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// ConfigNames return names for all configs in this Compose config
func (p Project) ConfigNames() []string {
	names := []string{}
	for k := range p.Configs {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// GetServices retrieve services by names, or return all services if no name specified
func (p Project) GetServices(names []string) (Services, error) {
	if len(names) == 0 {
		return p.Services, nil
	}
	services := Services{}
	for _, name := range names {
		service, err := p.GetService(name)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, nil
}

// GetService retrieve a specific service by name
func (p Project) GetService(name string) (ServiceConfig, error) {
	for _, s := range p.Services {
		if s.Name == name {
			return s, nil
		}
	}
	return ServiceConfig{}, fmt.Errorf("no such service: %s", name)
}

type ServiceFunc func(service ServiceConfig) error

// WithServices run ServiceFunc on each service and dependencies in dependency order
func (p Project) WithServices(names []string, fn ServiceFunc) error {
	return p.withServices(names, fn, map[string]bool{})
}

func (p Project) withServices(names []string, fn ServiceFunc, done map[string]bool) error {
	services, err := p.GetServices(names)
	if err != nil {
		return err
	}
	for _, service := range services {
		if done[service.Name] {
			continue
		}
		dependencies := service.GetDependencies()
		if len(dependencies) > 0 {
			err := p.withServices(dependencies, fn, done)
			if err != nil {
				return err
			}
		}
		if err := fn(service); err != nil {
			return err
		}
		done[service.Name] = true
	}
	return nil
}

// RelativePath resolve a relative path based project's working directory
func (p *Project) RelativePath(path string) string {
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.WorkingDir, path)
}
