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
	"reflect"
	"testing"

	"github.com/compose-spec/compose-go/v2/tree"
	"gotest.tools/v3/assert"
)

func Test_transformBuild(t *testing.T) {
	tests := []struct {
		name string
		yaml any
		want any
	}{
		{
			name: "single context string",
			yaml: "context_path",
			want: map[string]any{
				"context": "context_path",
			},
		},
		{
			name: "mapping without context",
			yaml: map[string]any{
				"dockerfile": "foo.Dockerfile",
			},
			want: map[string]any{
				"context":    ".",
				"dockerfile": "foo.Dockerfile",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformBuild(tt.yaml, tree.NewPath("services.foo.build"))
			assert.NilError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transformBuild() got = %v, want %v", got, tt.want)
			}
		})
	}
}
