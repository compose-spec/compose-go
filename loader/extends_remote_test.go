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
	"testing"

	"github.com/compose-spec/compose-go/v3/types"
	"go.yaml.in/yaml/v4"
	"gotest.tools/v3/assert"
)

// TestExtends_RemoteResource_PropagatesImage reproduces the docker/compose
// TestPublish OCI extends round-trip regression: a service that extends a
// service declared in a file fetched through a custom ResourceLoader
// (typically an OCI-style hashed path) must inherit the base service's
// fields, image: included.
func TestExtends_RemoteResource_PropagatesImage(t *testing.T) {
	tmpdir := t.TempDir()

	// Simulate an OCI-resolved file with a hashed-looking name.
	baseDir := filepath.Join(tmpdir, "abc123def456")
	assert.NilError(t, os.MkdirAll(baseDir, 0o755))
	basePath := filepath.Join(baseDir, "compose.yaml")
	assert.NilError(t, os.WriteFile(basePath, []byte(`
services:
  base:
    image: alpine:3.19
    user: root
`), 0o644))

	mainPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(mainPath, []byte(`
services:
  derived:
    extends:
      file: oci:hashed-image
      service: base
`), 0o644))

	p, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: mainPath}},
	}, withProjectName("test-extends-remote-image", true), func(opts *Options) {
		opts.SkipConsistencyCheck = true
		opts.SkipNormalization = true
		opts.ResourceLoaders = []ResourceLoader{
			ociMockLoader{root: baseDir},
		}
	})
	assert.NilError(t, err)

	derived, err := p.GetService("derived")
	assert.NilError(t, err)
	assert.Equal(t, derived.Image, "alpine:3.19")
	assert.Equal(t, derived.User, "root")
}

// TestExtends_RemoteResource_RoundTrip simulates a docker compose publish
// flow: load with an OCI-style resource loader, render the project as
// yaml, then reload that yaml without the loader. The derived service
// must still carry the image inherited from the remote base — after the
// extends key is dropped — so the second load can succeed without any
// remote fetch.
func TestExtends_RemoteResource_RoundTrip(t *testing.T) {
	tmpdir := t.TempDir()

	baseDir := filepath.Join(tmpdir, "deadbeefcafebabe")
	assert.NilError(t, os.MkdirAll(baseDir, 0o755))
	basePath := filepath.Join(baseDir, "compose.yaml")
	assert.NilError(t, os.WriteFile(basePath, []byte(`
services:
  base:
    image: redis:7
    command: ["redis-server"]
`), 0o644))

	mainPath := filepath.Join(tmpdir, "compose.yaml")
	assert.NilError(t, os.WriteFile(mainPath, []byte(`
services:
  cache:
    extends:
      file: oci:hashed-image
      service: base
    ports:
      - 6379:6379
`), 0o644))

	first, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: mainPath}},
	}, withProjectName("test-extends-roundtrip", true), func(opts *Options) {
		opts.SkipConsistencyCheck = true
		opts.SkipNormalization = true
		opts.ResourceLoaders = []ResourceLoader{
			ociMockLoader{root: baseDir},
		}
	})
	assert.NilError(t, err)

	cache := first.Services["cache"]
	assert.Equal(t, cache.Image, "redis:7", "image must be inherited from the remote base")
	assert.Assert(t, cache.Extends == nil || cache.Extends.File == "", "extends key must be cleared after resolution")

	// Round-trip through MarshalYAML and reload without the resource loader.
	// Without the OCI loader the reload would fail if extends were still
	// present in the rendered yaml, so the test fails if the round-trip
	// fails to drop extends or to inline the image.
	rendered, err := yaml.Marshal(first)
	assert.NilError(t, err)

	roundPath := filepath.Join(tmpdir, "round.yaml")
	assert.NilError(t, os.WriteFile(roundPath, rendered, 0o644))

	second, err := LoadWithContext(context.TODO(), types.ConfigDetails{
		WorkingDir:  tmpdir,
		ConfigFiles: []types.ConfigFile{{Filename: roundPath}},
	}, withProjectName("test-extends-roundtrip", true), func(opts *Options) {
		opts.SkipConsistencyCheck = true
		opts.SkipNormalization = true
	})
	assert.NilError(t, err)
	assert.Equal(t, second.Services["cache"].Image, "redis:7")
}

// ociMockLoader stands in for a buildkit/OCI-style ResourceLoader: any
// reference starting with "oci:" is mapped to <root>/compose.yaml so the
// loader has to deal with a path that doesn't look like the original
// reference at all.
type ociMockLoader struct {
	root string
}

func (l ociMockLoader) Accept(s string) bool {
	const prefix = "oci:"
	return len(s) > len(prefix) && s[:len(prefix)] == prefix
}

func (l ociMockLoader) Load(_ context.Context, _ string) (string, error) {
	return filepath.Join(l.root, "compose.yaml"), nil
}

func (l ociMockLoader) Dir(_ string) string {
	return l.root
}
