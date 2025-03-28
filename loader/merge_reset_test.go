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

package loader

import (
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func Test_LoadWithReset(t *testing.T) {
	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "base.yml",
				Content: []byte(`
 name: test
 services:
   foo:
     build:
       context: .
       dockerfile: foo.Dockerfile
     environment:
       FOO: BAR`),
			},
			{
				Filename: "override.yml",
				Content: []byte(`
 services:
   foo:
     image: foo
     build: !reset  
     environment:
       FOO: !reset
`),
			},
		},
	}, func(options *Options) {
		options.SkipNormalization = true
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, p.Services["foo"], types.ServiceConfig{
		Name:        "foo",
		Image:       "foo",
		Environment: types.MappingWithEquals{},
	})
}

func Test_DuplicateReset(t *testing.T) {
	_, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: "duplicate.yml",
				Content: []byte(`
 name: test
 services:
   foo:
     command: hello
     command: !reset hello world
`),
			},
		},
	}, func(options *Options) {
		options.SkipNormalization = true
	})
	assert.Error(t, err, "line 6: mapping key \"command\" already defined at line 5")
}
