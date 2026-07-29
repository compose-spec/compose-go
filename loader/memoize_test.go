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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

// writeDiamondInclude writes a doubling include graph of the given depth in dir:
// level<i>.yaml includes level<i+1>.yaml twice, the last level only defines a
// service. Without include memoization the leaf is expanded 2^depth times.
func writeDiamondInclude(t testing.TB, dir string, depth int) {
	t.Helper()
	for i := 0; i < depth; i++ {
		content := fmt.Sprintf(`
include:
  - path: level%[1]d.yaml
  - path: level%[1]d.yaml
services:
  svc%[2]d:
    image: busybox
`, i+1, i)
		assert.NilError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("level%d.yaml", i)), []byte(content), 0o600))
	}
	leaf := `
services:
  leaf:
    image: busybox
`
	assert.NilError(t, os.WriteFile(filepath.Join(dir, fmt.Sprintf("level%d.yaml", depth)), []byte(leaf), 0o600))
}

// TestIncludeDiamondDedup guards the include memoization: a depth-24 doubling
// include graph would require 2^24 leaf expansions without it and cannot
// complete within the timeout.
func TestIncludeDiamondDedup(t *testing.T) {
	dir := t.TempDir()
	writeDiamondInclude(t, dir, 24)

	type result struct {
		p   *types.Project
		err error
	}
	// The loader does not observe ctx.Done() today, so the timeout is consumed
	// by the select below, not by LoadWithContext.
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()

	done := make(chan result, 1)
	go func() {
		p, err := LoadWithContext(context.Background(), types.ConfigDetails{
			WorkingDir:  dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "level0.yaml")}},
		}, func(options *Options) {
			options.SetProjectName("diamond", true)
			options.SkipConsistencyCheck = true
			// Compose always registers a listener; make sure memoization stays
			// effective with one registered.
			options.Listeners = []Listener{func(string, map[string]any) {}}
		})
		done <- result{p, err}
	}()

	select {
	case r := <-done:
		assert.NilError(t, r.err)
		_, err := r.p.GetService("leaf")
		assert.NilError(t, err)
		_, err = r.p.GetService("svc23")
		assert.NilError(t, err)
	case <-timer.C:
		t.Fatal("diamond include did not complete within 20s — include memoization likely not working")
	}
}

// TestIncludeListenerTopLevelOnly pins the Listener contract: include/extends
// events are only emitted for declarations in the config files passed to the
// loader, not for ones inside included files.
func TestIncludeListenerTopLevelOnly(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"compose.yaml": `
name: top-level-listener
include:
  - path: child-a.yaml
  - path: child-b.yaml
services:
  app:
    image: app
    extends: base
  base:
    image: busybox
`,
		"child-a.yaml": `
include:
  - path: shared.yaml
services:
  child-a-svc:
    image: busybox
    extends: child-a-base
  child-a-base:
    image: alpine
`,
		"child-b.yaml": `
include:
  - path: shared.yaml
services:
  child-b-svc:
    image: busybox
`,
		"shared.yaml": `
services:
  shared:
    image: shared
`,
	}
	for name, content := range files {
		assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	includeCount, extendsCount := 0, 0
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
	}, func(options *Options) {
		options.SkipConsistencyCheck = true
		options.Listeners = []Listener{
			func(event string, _ map[string]any) {
				switch event {
				case "include":
					includeCount++
				case "extends":
					extendsCount++
				}
			},
		}
	})
	assert.NilError(t, err)

	for _, name := range []string{"app", "base", "child-a-svc", "child-a-base", "child-b-svc", "shared"} {
		_, err := p.GetService(name)
		assert.NilError(t, err)
	}
	// only the two include declarations of compose.yaml, not the nested ones
	assert.Equal(t, includeCount, 2)
	// only app's extends, not child-a-svc's
	assert.Equal(t, extendsCount, 1)
}

// TestIncludeSameFileDistinctEnv guards the include cache key: the same file
// included twice with different env_file values must not share a memoized
// model. Distinct label keys are derived from the environment so both
// expansions stay observable after the conflict merge.
func TestIncludeSameFileDistinctEnv(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"compose.yaml": `
name: include-env-isolation
include:
  - path: shared.yaml
    env_file: a.env
  - path: shared.yaml
    env_file: b.env
`,
		"shared.yaml": `
services:
  shared:
    image: busybox
    labels:
      - tag-${TAG:-def}=1
`,
		"a.env": "TAG=a\n",
		"b.env": "TAG=b\n",
	}
	for name, content := range files {
		assert.NilError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	}

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
	}, func(options *Options) {
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	shared, err := p.GetService("shared")
	assert.NilError(t, err)
	assert.Equal(t, shared.Labels["tag-a"], "1")
	assert.Equal(t, shared.Labels["tag-b"], "1")
}

// TestExtendsFileNameCollision guards the extends.file cache against sharing a
// mutable services map: resolving service web (same name as its base in
// base.yaml) writes the merged result back into the base services map, which
// must not leak into the other services extending the same base.
func TestExtendsFileNameCollision(t *testing.T) {
	dir := t.TempDir()
	base := `
services:
  web:
    image: base
`
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(base), 0o600))

	root := `
name: extends-collision
services:
  web:
    extends:
      file: base.yaml
      service: web
    environment:
      - POLLUTED=1
`
	for i := 2; i <= 5; i++ {
		root += fmt.Sprintf(`
  web%d:
    extends:
      file: base.yaml
      service: web
`, i)
	}
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(root), 0o600))

	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		WorkingDir:  dir,
		ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
	}, func(options *Options) {
		options.SkipConsistencyCheck = true
	})
	assert.NilError(t, err)

	web, err := p.GetService("web")
	assert.NilError(t, err)
	assert.Check(t, web.Environment["POLLUTED"] != nil)
	for i := 2; i <= 5; i++ {
		svc, err := p.GetService(fmt.Sprintf("web%d", i))
		assert.NilError(t, err)
		assert.Equal(t, svc.Image, "base")
		_, polluted := svc.Environment["POLLUTED"]
		assert.Check(t, !polluted, "web%d inherited environment from the extending service web", i)
	}
}

// TestExtendsCacheServesIsolatedCopies deterministically checks that a cache
// hit returns a services map that is not aliased to the stored entry.
func TestExtendsCacheServesIsolatedCopies(t *testing.T) {
	dir := t.TempDir()
	base := `
services:
  base:
    image: busybox
`
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(base), 0o600))

	ctx := withExtendsCache(context.Background())
	opts := &Options{
		SkipInterpolation: true,
		ResourceLoaders:   []ResourceLoader{localResourceLoader{WorkingDir: dir}},
	}
	first, _, err := getExtendsBaseFromFile(ctx, "svc", "base", "compose.yaml", "base.yaml", opts, &cycleTracker{})
	assert.NilError(t, err)
	second, _, err := getExtendsBaseFromFile(ctx, "svc", "base", "compose.yaml", "base.yaml", opts, &cycleTracker{})
	assert.NilError(t, err)
	assert.DeepEqual(t, first, second)

	// simulate the write-back done by applyServiceExtends on the first copy
	first["base"].(map[string]any)["image"] = "mutated"
	assert.Equal(t, second["base"].(map[string]any)["image"], "busybox")

	third, _, err := getExtendsBaseFromFile(ctx, "svc", "base", "compose.yaml", "base.yaml", opts, &cycleTracker{})
	assert.NilError(t, err)
	assert.Equal(t, third["base"].(map[string]any)["image"], "busybox")
}

func BenchmarkIncludeDiamond(b *testing.B) {
	dir := b.TempDir()
	writeDiamondInclude(b, dir, 16)
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := LoadWithContext(context.Background(), types.ConfigDetails{
			WorkingDir:  dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "level0.yaml")}},
		}, func(options *Options) {
			options.SetProjectName("diamond", true)
			options.SkipConsistencyCheck = true
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtendsFile(b *testing.B) {
	dir := b.TempDir()
	base := `
services:
  base:
    image: busybox
    environment:
      - FOO=bar
`
	if err := os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(base), 0o600); err != nil {
		b.Fatal(err)
	}
	root := "name: extends-bench\nservices:"
	for i := 0; i < 40; i++ {
		root += fmt.Sprintf(`
  svc%d:
    extends:
      file: base.yaml
      service: base
`, i)
	}
	if err := os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte(root), 0o600); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_, err := LoadWithContext(context.Background(), types.ConfigDetails{
			WorkingDir:  dir,
			ConfigFiles: []types.ConfigFile{{Filename: filepath.Join(dir, "compose.yaml")}},
		}, func(options *Options) {
			options.SkipConsistencyCheck = true
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
