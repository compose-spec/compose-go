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
	"go.yaml.in/yaml/v4"
)

// HealthCheckConfig the healthcheck configuration for a service
type HealthCheckConfig struct {
	Test          HealthCheckTest `yaml:"test,omitempty" json:"test,omitempty"`
	Timeout       *Duration       `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Interval      *Duration       `yaml:"interval,omitempty" json:"interval,omitempty"`
	Retries       *uint64         `yaml:"retries,omitempty" json:"retries,omitempty"`
	StartPeriod   *Duration       `yaml:"start_period,omitempty" json:"start_period,omitempty"`
	StartInterval *Duration       `yaml:"start_interval,omitempty" json:"start_interval,omitempty"`
	Disable       bool            `yaml:"disable,omitempty" json:"disable,omitempty"`

	Extensions Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

// HealthCheckTest is the command run to test the health of a service
type HealthCheckTest []string

func (l *HealthCheckTest) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	switch node.Kind {
	case yaml.ScalarNode:
		*l = []string{"CMD-SHELL", node.Value}
	case yaml.SequenceNode:
		seq := make([]string, len(node.Content))
		for i, item := range node.Content {
			seq[i] = item.Value
		}
		*l = seq
	default:
		return NodeErrorf(node, "unexpected node kind %d for healthcheck.test", node.Kind)
	}
	return nil
}
