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

package transform

import (
	"testing"

	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// Canonical turns the short syntaxes of a model into their canonical form, and
// reports a short syntax holding values of the wrong type as an error naming
// the attribute: a list expected to hold strings (service names, networks,
// key=value pairs) must never crash it, whether or not the model was validated
// against the schema.
func TestCanonicalRejectsListsOfNonStrings(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr string
	}{
		{
			name:    "depends_on entry",
			model:   "services:\n  foo:\n    depends_on: [db, 3]\n",
			wantErr: "services.foo.depends_on: invalid type int for a depends_on entry",
		},
		{
			name:    "networks entry",
			model:   "services:\n  foo:\n    networks: [front, true]\n",
			wantErr: "services.foo.networks: invalid type bool, expected a string",
		},
		{
			name:    "models entry",
			model:   "services:\n  foo:\n    models: [llm, null]\n",
			wantErr: "services.foo.models: invalid type <nil>, expected a string",
		},
		{
			name:    "additional_contexts entry",
			model:   "services:\n  foo:\n    build:\n      additional_contexts: [3]\n",
			wantErr: "services.foo.build.additional_contexts: invalid type int, expected a key=value string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var model map[string]any
			assert.NilError(t, yaml.Unmarshal([]byte(tt.model), &model))
			_, err := Canonical(model, false)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
