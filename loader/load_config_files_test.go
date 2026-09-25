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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// fakeRemoteLoader simulates a remote resource loader (git, oci): it hands
// back a path under its own "download" directory, exactly like a real
// remote loader returns the local copy it fetched the artifact into.
type fakeRemoteLoader struct {
	downloadDir string
}

func (l fakeRemoteLoader) Accept(path string) bool {
	return strings.HasPrefix(path, "remote://")
}

func (l fakeRemoteLoader) Load(_ context.Context, _ string) (string, error) {
	return filepath.Join(l.downloadDir, "compose.yaml"), nil
}

func (l fakeRemoteLoader) Dir(_ string) string {
	return l.downloadDir
}

// TestLoadConfigFilesHonorsExplicitWorkingDir is the docker/compose#14224
// repro at the compose-go layer: an explicit working dir (--project-directory)
// must survive a remote resource loader, which otherwise defaults the
// working dir to its own downloaded copy's directory so a self-contained
// remote artifact (extends, bundled env files) resolves against itself.
// That default must never override an explicit request.
func TestLoadConfigFilesHonorsExplicitWorkingDir(t *testing.T) {
	downloadDir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(downloadDir, "compose.yaml"), []byte("services: {}"), 0o600))
	remote := fakeRemoteLoader{downloadDir: downloadDir}

	t.Run("explicit working dir is preserved", func(t *testing.T) {
		explicitDir := t.TempDir()
		config, err := LoadConfigFiles(context.Background(), []string{"remote://ref"}, explicitDir,
			func(o *Options) { o.ResourceLoaders = []ResourceLoader{remote} },
			func(o *Options) { o.SetWorkingDirExplicit(true) },
		)
		assert.NilError(t, err)
		assert.Equal(t, config.WorkingDir, explicitDir)
	})

	t.Run("defaulted working dir still falls back to the downloaded copy's directory", func(t *testing.T) {
		defaultedDir := t.TempDir()
		config, err := LoadConfigFiles(context.Background(), []string{"remote://ref"}, defaultedDir,
			func(o *Options) { o.ResourceLoaders = []ResourceLoader{remote} },
		)
		assert.NilError(t, err)
		assert.Equal(t, config.WorkingDir, downloadDir)
	})
}
