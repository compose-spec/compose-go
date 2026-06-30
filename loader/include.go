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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/compose-spec/compose-go/v2/dotenv"
	interp "github.com/compose-spec/compose-go/v2/interpolation"
	"github.com/compose-spec/compose-go/v2/override"
	"github.com/compose-spec/compose-go/v2/tree"
	"github.com/compose-spec/compose-go/v2/types"
)

// includeCache memoizes loaded include models for the duration of a single
// project load. A file reached through more than one include path (a "diamond"
// in the include graph) was previously parsed and recursively expanded once per
// path, which is quadratic-to-exponential on deep graphs. The cache parses and
// expands each distinct include only once.
//
// The key captures everything that determines the loaded model — the resolved
// file paths, the project directory, and the effective environment — so a cache
// hit is equivalent to a fresh load even when the same file is included with a
// different env_file or project_directory. Each consumer gets a fresh deep copy,
// so importResources (and later normalization) never mutates a cached entry or a
// sibling branch that shares it.
//
// Cycle-safe: an include cycle is intrinsic to a node's subtree (the back-edge
// is in the fixed set of files the node includes), so it is detected on the
// node's first load, which fails before the node can be cached.
//
// Listener fidelity: loadYamlModel emits listener events (notably "extends", and
// nested "include") for the subtree it expands. Memoizing that call would drop
// those events on every cache hit, silently changing the public Listener
// contract (e.g. an extends-counting consumer would see a count that depends on
// include topology). Each entry therefore records the events emitted on first
// load and replays them, in order, on every cache hit — see ApplyInclude.
type includeCache struct {
	mu      sync.Mutex
	entries map[string]includeCacheEntry
}

// includeCacheEntry is a memoized include: the expanded model plus the listener
// events emitted while expanding it, replayed on cache hits to keep per-include
// listeners firing once per occurrence.
type includeCacheEntry struct {
	model  map[string]any
	events []recordedEvent
}

// recordedEvent is a single listener event captured during an include load so it
// can be replayed verbatim on a later cache hit.
type recordedEvent struct {
	event    string
	metadata map[string]any
}

type includeCacheKey struct{}

// getOrCreateIncludeCache returns the include cache carried by ctx, creating one
// (and a derived context) on first use so that all sibling and descendant
// includes of a single load share it.
func getOrCreateIncludeCache(ctx context.Context) (*includeCache, context.Context) {
	if c, ok := ctx.Value(includeCacheKey{}).(*includeCache); ok {
		return c, ctx
	}
	c := &includeCache{entries: map[string]includeCacheEntry{}}
	return c, context.WithValue(ctx, includeCacheKey{}, c)
}

func (c *includeCache) get(key string) (includeCacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		return includeCacheEntry{model: deepCopyMapping(e.model), events: e.events}, true
	}
	return includeCacheEntry{}, false
}

func (c *includeCache) put(key string, model map[string]any, events []recordedEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = includeCacheEntry{model: deepCopyMapping(model), events: events}
}

// includeKey hashes the inputs that determine an included model. Two include
// entries with the same key load identical content — including identical
// relative paths, so a cached model is reuse-safe in the caller's context.
//
// workingDir (the relative base the included model's paths are resolved against)
// is part of the key: the same file reached through two include parents can have
// a different relative base (e.g. "a/b" vs "b"), which yields models with
// different relative paths. Keying on it avoids reusing a model whose paths the
// caller would then rebase incorrectly.
//
// Encoding: every field is length-prefixed and each variable-length section is
// count-prefixed. A bare separator byte is not safe here — types.Mapping is
// populated from .env files / process env, where any byte (NUL included) is a
// legal key or value, so a value could otherwise impersonate a separator and
// two distinct (paths, env) tuples could hash to the same key, serving a wrong
// cached model with no error surfaced. Length/count prefixes make the byte
// stream uniquely decodable, so distinct inputs always produce distinct keys.
//
// Note: Substitute and TypeCastMapping are intentionally excluded from the key.
// They are invariant across includes within a single Load (cloned unchanged from
// the top-level options at the call site). If a future option lets them vary per
// include, they must be folded into the key.
func includeKey(paths []string, workingDir, projectDir string, env types.Mapping) string {
	h := sha256.New()
	writeInt := func(n int) { _, _ = fmt.Fprintf(h, "%d:", n) }
	write := func(s string) {
		_, _ = fmt.Fprintf(h, "%d:", len(s))
		_, _ = h.Write([]byte(s))
	}
	writeInt(len(paths))
	for _, p := range paths {
		write(p)
	}
	write(workingDir)
	write(projectDir)
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	writeInt(len(keys))
	for _, k := range keys {
		write(k)
		write(env[k])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// deepCopyMapping returns a deep copy of a generic YAML mapping (the shape of a
// not-yet-typed compose model: nested map[string]any / []any / scalars).
func deepCopyMapping(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return deepCopyMapping(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = deepCopyValue(e)
		}
		return out
	default:
		return v
	}
}

// loadIncludeConfig parse the required config from raw yaml
func loadIncludeConfig(source any) ([]types.IncludeConfig, error) {
	if source == nil {
		return nil, nil
	}
	configs, ok := source.([]any)
	if !ok {
		return nil, fmt.Errorf("`include` must be a list, got %s", source)
	}
	for i, config := range configs {
		if v, ok := config.(string); ok {
			configs[i] = map[string]any{
				"path": v,
			}
		}
	}
	var requires []types.IncludeConfig
	err := Transform(source, &requires)
	return requires, err
}

func ApplyInclude(ctx context.Context, workingDir string, environment types.Mapping, model map[string]any, options *Options, included []string, processor PostProcessor) error {
	includeConfig, err := loadIncludeConfig(model["include"])
	if err != nil {
		return err
	}

	cache, ctx := getOrCreateIncludeCache(ctx)

	for _, r := range includeConfig {
		for _, listener := range options.Listeners {
			listener("include", map[string]any{
				"path":       r.Path,
				"workingdir": workingDir,
			})
		}

		var relworkingdir string
		for i, p := range r.Path {
			for _, loader := range options.ResourceLoaders {
				if !loader.Accept(p) {
					continue
				}
				path, err := loader.Load(ctx, p)
				if err != nil {
					return err
				}
				p = path

				if i == 0 { // This is the "main" file, used to define project-directory. Others are overrides

					switch {
					case r.ProjectDirectory == "":
						relworkingdir = loader.Dir(path)
						r.ProjectDirectory = filepath.Dir(path)
					case !filepath.IsAbs(r.ProjectDirectory):
						relworkingdir = loader.Dir(r.ProjectDirectory)
						r.ProjectDirectory = filepath.Join(workingDir, r.ProjectDirectory)

					default:
						relworkingdir = r.ProjectDirectory

					}
					for _, f := range included {
						if f == path {
							included = append(included, path)
							return fmt.Errorf("include cycle detected:\n%s\n include %s", included[0], strings.Join(included[1:], "\n include "))
						}
					}
				}
			}
			r.Path[i] = p
		}

		loadOptions := options.clone()
		loadOptions.ResolvePaths = true
		loadOptions.SkipNormalization = true
		loadOptions.SkipConsistencyCheck = true
		loadOptions.ResourceLoaders = append(loadOptions.RemoteResourceLoaders(), localResourceLoader{
			WorkingDir: r.ProjectDirectory,
		})

		if len(r.EnvFile) == 0 {
			f := filepath.Join(r.ProjectDirectory, ".env")
			if s, err := os.Stat(f); err == nil && !s.IsDir() {
				r.EnvFile = types.StringList{f}
			}
		} else {
			envFile := []string{}
			for _, f := range r.EnvFile {
				if f == "/dev/null" {
					continue
				}
				if !filepath.IsAbs(f) {
					f = filepath.Join(workingDir, f)
					s, err := os.Stat(f)
					if err != nil {
						return err
					}
					if s.IsDir() {
						return fmt.Errorf("%s is not a file", f)
					}
				}
				envFile = append(envFile, f)
			}
			r.EnvFile = envFile
		}

		envFromFile, err := dotenv.GetEnvFromFile(environment, r.EnvFile)
		if err != nil {
			return err
		}

		config := types.ConfigDetails{
			WorkingDir:  relworkingdir,
			ConfigFiles: types.ToConfigFiles(r.Path),
			Environment: environment.Clone().Merge(envFromFile),
		}
		loadOptions.Interpolate = &interp.Options{
			Substitute:      options.Interpolate.Substitute,
			LookupValue:     config.LookupEnv,
			TypeCastMapping: options.Interpolate.TypeCastMapping,
		}
		// Memoize by the inputs that determine the loaded model so a file
		// reached through several include paths is parsed and expanded once.
		// The merge into `model` still runs for every occurrence (a copy is
		// handed out), so any same-file `extends` in the including file still
		// resolves and the result is identical to loading it each time.
		key := includeKey(r.Path, config.WorkingDir, r.ProjectDirectory, config.Environment)
		entry, ok := cache.get(key)
		switch {
		case ok:
			// Replay the events the first load emitted so per-occurrence
			// listeners (e.g. "extends", nested "include") fire for this
			// traversal too — a cache hit must be indistinguishable from a
			// fresh load to a listener. A fresh copy of each metadata map is
			// handed out, matching the per-load isolation of a real expansion.
			for _, ev := range entry.events {
				options.ProcessEvent(ev.event, deepCopyMapping(ev.metadata))
			}
		case len(loadOptions.Listeners) == 0:
			// No listeners: there is no event contract to preserve, so skip
			// recording entirely. This keeps memoization cheap on the common
			// listener-free path; recording here would make a doubling include
			// graph emit (and replay) an exponential number of internal events,
			// reintroducing the very blow-up the cache exists to remove.
			entry.model, err = loadYamlModel(ctx, config, loadOptions, &cycleTracker{}, included)
			if err != nil {
				return err
			}
			cache.put(key, entry.model, nil)
		default:
			// Record every event emitted while expanding this subtree so it can
			// be replayed on later cache hits. The recorder runs alongside the
			// real listeners, which still fire live on this first occurrence;
			// nested includes clone these options and so feed this recorder too,
			// making the recording the subtree's full event stream.
			var recorded []recordedEvent
			loadOptions.Listeners = append(append([]Listener{}, loadOptions.Listeners...),
				func(event string, metadata map[string]any) {
					recorded = append(recorded, recordedEvent{event: event, metadata: deepCopyMapping(metadata)})
				})
			entry.model, err = loadYamlModel(ctx, config, loadOptions, &cycleTracker{}, included)
			if err != nil {
				return err
			}
			cache.put(key, entry.model, recorded)
		}
		err = importResources(entry.model, model, processor)
		if err != nil {
			return err
		}
	}
	delete(model, "include")
	return nil
}

// importResources import into model all resources defined by imported, and report error on conflict
func importResources(source map[string]any, target map[string]any, processor PostProcessor) error {
	if err := importResource(source, target, "services", processor); err != nil {
		return err
	}
	if err := importResource(source, target, "volumes", processor); err != nil {
		return err
	}
	if err := importResource(source, target, "networks", processor); err != nil {
		return err
	}
	if err := importResource(source, target, "secrets", processor); err != nil {
		return err
	}
	if err := importResource(source, target, "configs", processor); err != nil {
		return err
	}
	if err := importResource(source, target, "models", processor); err != nil {
		return err
	}
	return nil
}

func importResource(source map[string]any, target map[string]any, key string, processor PostProcessor) error {
	from := source[key]
	if from != nil {
		var to map[string]any
		if v, ok := target[key]; ok {
			to = v.(map[string]any)
		} else {
			to = map[string]any{}
		}
		for name, a := range from.(map[string]any) {
			conflict, ok := to[name]
			if !ok {
				to[name] = a
				continue
			}
			err := processor.Apply(map[string]any{
				key: map[string]any{
					name: a,
				},
			})
			if err != nil {
				return err
			}

			merged, err := override.MergeYaml(a, conflict, tree.NewPath(key, name))
			if err != nil {
				return err
			}
			to[name] = merged
		}
		target[key] = to
	}
	return nil
}
