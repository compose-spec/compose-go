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

// ServiceModels is a map of model names to service model configurations.
// It supports both list syntax (models: [foo]) and map syntax.
type ServiceModels map[string]*ServiceModelConfig

func (m *ServiceModels) UnmarshalYAML(value *yaml.Node) error {
	node := resolveYAMLNode(value)
	switch node.Kind {
	case yaml.SequenceNode:
		models := make(ServiceModels, len(node.Content))
		for _, item := range node.Content {
			models[item.Value] = nil
		}
		*m = models
	case yaml.MappingNode:
		type plain ServiceModels
		var p plain
		if err := node.Decode(&p); err != nil {
			return err
		}
		*m = ServiceModels(p)
	default:
		return fmt.Errorf("models must be a mapping or sequence, got %v", node.Kind)
	}
	return nil
}

type ModelConfig struct {
	Name         string     `yaml:"name,omitempty" json:"name,omitempty"`
	Model        string     `yaml:"model,omitempty" json:"model,omitempty"`
	ContextSize  int        `yaml:"context_size,omitempty" json:"context_size,omitempty"`
	RuntimeFlags []string   `yaml:"runtime_flags,omitempty" json:"runtime_flags,omitempty"`
	Extensions   Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}

type ServiceModelConfig struct {
	EndpointVariable string     `yaml:"endpoint_var,omitempty" json:"endpoint_var,omitempty"`
	ModelVariable    string     `yaml:"model_var,omitempty" json:"model_var,omitempty"`
	Extensions       Extensions `yaml:"#extensions,inline,omitempty" json:"-"`
}
