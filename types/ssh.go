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

type SSHKey struct {
	ID   string `yaml:"id,omitempty" json:"id,omitempty"`
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}

// SSHConfig is a mapping type for SSH build config
type SSHConfig []SSHKey

func (s SSHConfig) Get(id string) (string, error) {
	for _, sshKey := range s {
		if sshKey.ID == id {
			return sshKey.Path, nil
		}
	}
	return "", fmt.Errorf("ID %s not found in SSH keys", id)
}

// MarshalYAML makes SSHKey implement yaml.Marshaller
func (s SSHKey) MarshalYAML() (interface{}, error) {
	if s.Path == "" {
		return s.ID, nil
	}
	return fmt.Sprintf("%s: %s", s.ID, s.Path), nil
}

// MarshalJSON makes SSHKey implement json.Marshaller
func (s SSHKey) MarshalJSON() ([]byte, error) {
	if s.Path == "" {
		return []byte(fmt.Sprintf(`%q`, s.ID)), nil
	}
	return []byte(fmt.Sprintf(`%q: %s`, s.ID, s.Path)), nil
}

// UnmarshalYAML accepts a canonical mapping of `id: path` entries (the
// short-form `default` and `id=path` forms are turned into this shape by
// transform.CanonicalNode before decoding) and stores them as a slice of
// SSHKey. Mirrors DecodeMapstructure for yaml.v4 native decoding.
func (s *SSHConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("invalid ssh config type, expected mapping, got %v", value.Kind)
	}
	result := make(SSHConfig, 0, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := SSHKey{ID: value.Content[i].Value}
		if v := value.Content[i+1]; v.Kind == yaml.ScalarNode && v.Tag != "!!null" {
			key.Path = v.Value
		}
		result = append(result, key)
	}
	*s = result
	return nil
}
