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

package tests

import (
	"context"
	"testing"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestDevelopWatch(t *testing.T) {
	p, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  web:
    image: app
    build: ./app
    develop:
      watch:
        - path: ./src
          action: sync
          target: /app/src
          ignore:
            - node_modules/
          initial_sync: true
        - path: ./requirements.txt
          action: rebuild
        - path: ./config.yml
          action: sync+restart
          target: /app/config.yml
`)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.ResolvePaths = false
		options.SkipValidation = true
	})
	assert.NilError(t, err)
	dev := p.Services["web"].Develop
	assert.Assert(t, dev != nil)

	assert.Equal(t, len(dev.Watch), 3)

	assert.Equal(t, dev.Watch[0].Path, "./src")
	assert.Equal(t, dev.Watch[0].Action, types.WatchActionSync)
	assert.Equal(t, dev.Watch[0].Target, "/app/src")
	assert.DeepEqual(t, dev.Watch[0].Ignore, []string{"node_modules/"})
	assert.Equal(t, dev.Watch[0].InitialSync, true)

	assert.Equal(t, dev.Watch[1].Path, "./requirements.txt")
	assert.Equal(t, dev.Watch[1].Action, types.WatchActionRebuild)

	assert.Equal(t, dev.Watch[2].Path, "./config.yml")
	assert.Equal(t, dev.Watch[2].Action, types.WatchActionSyncRestart)
	assert.Equal(t, dev.Watch[2].Target, "/app/config.yml")
}

func TestDevelopMissingAction(t *testing.T) {
	_, err := loader.LoadWithContext(context.TODO(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{{Filename: "compose.yml", Content: []byte(`
name: test
services:
  web:
    image: app
    build: ./app
    develop:
      watch:
        - path: ./src
          target: /app/src
`)}},
		Environment: map[string]string{},
	}, func(options *loader.Options) {
		options.ResolvePaths = false
	})
	assert.ErrorContains(t, err, "services.web.develop.watch.0 missing property 'action'")
}
