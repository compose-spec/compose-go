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

func Test_transformUlimits(t *testing.T) {
	tests := []struct {
		name             string
		yaml             any
		ignoreParseError bool
		want             any
		wantErr          string
	}{
		{
			name: "int",
			yaml: 65535,
			want: 65535,
		},
		{
			name: "long syntax",
			yaml: map[string]any{
				"soft": 20000,
				"hard": 40000,
			},
			want: map[string]any{
				"soft": 20000,
				"hard": 40000,
			},
		},
		{
			name:    "unresolved variable, error",
			yaml:    "${NOFILE}",
			wantErr: `test: invalid type string for ulimits`,
		},
		{
			name:             "unresolved variable, ignored",
			yaml:             "${NOFILE}",
			ignoreParseError: true,
			want:             "${NOFILE}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformUlimits(tt.yaml, tree.NewPath("test"), tt.ignoreParseError)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transformUlimits() got = %v, want %v", got, tt.want)
			}
		})
	}
}
