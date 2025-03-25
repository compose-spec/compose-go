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

package schema

import (
	// Enable support for embedded static resources
	_ "embed"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schema is the compose-spec JSON schema
//
//go:embed compose-spec.json
var Schema string

// Validate uses the jsonschema to validate the configuration
func Validate(config map[string]interface{}) error {
	schemaLoader := jsonschema.NewCompiler()
	err := schemaLoader.AddResource("compose_spec.json", Schema)
	if err != nil {
		return err
	}

	schema, err := schemaLoader.Compile("compose_spec.json")
	if err != nil {
		return err
	}

	return schema.Validate(config)
}
