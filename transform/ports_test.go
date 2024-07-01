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

func Test_transformPorts(t *testing.T) {
	tests := []struct {
		name             string
		yaml             any
		ignoreParseError bool
		want             any
		wantErr          string
	}{
		{
			name: "[]string",
			yaml: []any{
				"127.0.0.1:8080:80",
				"127.0.0.1:8081:81",
			},
			want: []any{
				map[string]any{
					"host_ip":   "127.0.0.1",
					"mode":      "ingress",
					"protocol":  "tcp",
					"published": "8080",
					"target":    uint32(80),
				},
				map[string]any{
					"host_ip":   "127.0.0.1",
					"mode":      "ingress",
					"protocol":  "tcp",
					"published": "8081",
					"target":    uint32(81),
				},
			},
		},
		{
			name: "invalid IP",
			yaml: []any{
				"127.0.1:8080:80",
			},
			wantErr: "Invalid ip address: 127.0.1",
		},
		{
			name: "ignore invalid IP",
			yaml: []any{
				"${HOST_IP}:${HOST_PORT}:80",
			},
			ignoreParseError: true,
			want: []any{
				"${HOST_IP}:${HOST_PORT}:80",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transformPorts(tt.yaml, tree.NewPath("services.foo.ports"), tt.ignoreParseError)
			if tt.wantErr != "" {
				assert.Error(t, err, tt.wantErr)
				return
			}
			assert.NilError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("transformPorts() got = %v, want %v", got, tt.want)
			}
		})
	}
}
