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

// JobConfig is the configuration of one job
type JobConfig struct {
	Name     string         `yaml:"name,omitempty" json:"-"`
	Triggers *TriggerConfig `yaml:"triggers,omitempty" json:"triggers,omitempty"`

	ContainerSpec `yaml:",inline" mapstructure:",squash"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

// TriggerConfig defines trigger conditions for a job
type TriggerConfig struct {
	Schedule   string     `yaml:"schedule,omitempty" json:"schedule,omitempty"`
	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

// Jobs is a mapping of job names to job configurations
type Jobs map[string]JobConfig
