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

type EnvFile struct {
	Path     string `yaml:"path,omitempty" json:"path,omitempty"`
	Required OptOut `yaml:"required,omitempty" json:"required,omitzero"`
	Format   string `yaml:"format,omitempty" json:"format,omitempty"`

	// Context preserves the yaml loading context for this entry:
	//   - WorkingDir: base directory to resolve Path
	//   - Env:        variables available to interpolate the file content,
	//                 including variables provided by an enclosing
	//                 include.env_file
	//   - Source:     yaml file where this env_file entry was declared
	//
	// Populated by the loader when running through the yaml.Node based
	// pipeline. Excluded from YAML and JSON serialization.
	Context *NodeContext `yaml:"-" json:"-"`
}
