# compose-go loading pipeline

This document describes how `compose-go` turns one or more Compose YAML
files into a `*types.Project`. It targets contributors who need to
extend the loader, debug an unexpected merge result, or understand why
a given fixture surfaces a particular value.

## Goals

The loader is built around three invariants:

1. **A YAML node never loses its origin.** Every scalar carries an
   implicit "where did this come from" — file path, working directory,
   environment of the layer that introduced it. The pipeline preserves
   that link so interpolation, path resolution and diagnostics can
   reason about it per scalar.
2. **Lazy resolution at the right scope.** Variables, `env_file`
   contents, `extends` targets and `include` directives are resolved in
   the scope of the layer that declared them, not the global project
   scope. A variable introduced by an include's `env_file` is visible
   to the scalars from that include and to nothing else.
3. **One canonical merged tree.** All files end up in a single
   `*yaml.Node` tree that two helpers project into either a
   `map[string]any` (for `LoadModelWithContext`) or a typed
   `*types.Project` (for `LoadWithContext`).

The implementation stays in the `loader` package; supporting helpers
live in `internal/node`, `paths`, `override`, `interpolation`,
`transform`, `validation` and `normalize`.

## Public entry points

```go
loader.LoadWithContext(ctx, configDetails, opts...)      (*types.Project, error)
loader.LoadModelWithContext(ctx, configDetails, opts...) (map[string]any, error)
```

Both share the same `Options` struct, run the same pipeline, and differ
only in their final projection. There is no other supported way to drive
the loader — internal helpers (`load`, `nodeToProject`, `nodeToModel`) are
not exported.

## Core types

The pipeline works on a small set of values that survive across phases:

- **`internal/node.Layer`** holds one parsed compose document. It wraps
  the document's `*yaml.Node` together with the `SourceContext` that
  describes where the document came from. A layer is the unit of merge
  and the unit of resource scope.
- **`internal/node.SourceContext`** records, for one layer, the absolute
  file path, the working directory used to resolve its relative paths,
  the environment in effect during its load, and the `env_file` entries
  that contributed to that environment. It also carries a pointer to its
  parent context so include / extends chains can walk back to the top.
- **`origins map[*yaml.Node]*node.SourceContext`** is a side-table
  populated after every layer has been parsed and after extends have run
  in place. It attributes every reachable node to the layer that
  produced it; the path resolver, the environment resolver and the
  interpolation step all consult it to pick the right scope for each
  scalar.

## Pipeline

`load(ctx, cd, opts)` in `loader/load.go` is the single orchestrator. It
runs the steps below; numbers match the doc comment at the top of the
function.

### 1. Parse + per-document hygiene

For every `ConfigFile` in `ConfigDetails`, `LoadLayer` (see
`loader/load_layer.go`):

- reads the YAML into one or more documents (`---` separated),
- runs `internal/node.ResolveResetOverride` to extract every `!reset`
  and `!override` path while removing the tags from the tree,
- runs `internal/node.NormalizeAliases` to unfold anchors / merge keys
  (with a cap on total deep-copies to defend against alias bombs),
- runs `checkStringKeys` to surface non-string mapping keys with a
  precise diagnostic,
- wraps the resulting tree in a `Layer` with its `SourceContext`.

### 2. Recursive `include`

`CollectIncludeLayers` (in `loader/load_include.go`) reads the top-level
`include:` block on a layer, interpolates only that block in the parent
environment, then for each entry:

- resolves the `path` through the registered `ResourceLoader`s,
- computes the include's `project_directory` (absolute) and loads the
  declared `env_file`s on top of the parent environment,
- recursively expands its own includes (cycle detected via the absolute
  filename chain in `expandIncludes`),
- (when `opts.ResolvePaths` is on) runs a sub-resolve so paths inside
  the included file are absolutized in the include's working directory.

The included layers are appended in order so the parent's overrides win
later in the merge.

### 3. `extends`

`applyExtendsPerLayer` (in `loader/load.go`) iterates every service in
every layer. For each `extends:` directive:

- `parseExtendsRef` extracts the `(service, file)` tuple and emits the
  `extends` Listener event,
- `loadExtendsBaseLayer` loads the base file if needed, with a child
  `Options` whose `ResourceLoaders` is re-rooted at the base file's
  directory,
- the chain recurses (cycle tracked by `cycleTracker.Add(file, service)`),
- the base service is `deepCloneNode`d, `!reset`/`!override` paths from
  the current layer are pre-applied to the clone, then
  `override.MergeNode` merges base + derived under the canonical
  `services.x` path,
- `resolveExtendedServicePaths` runs path resolution on the merged
  service against the v2-compatible relative WorkingDir carried in
  `Options.extendsRelativeDir`, so cross-file extends accumulate the
  expected relative form.

`extends` mutates the layer in place — the merged service body
replaces its original entry inside the layer's tree.

### 4. Origins side-table

`populateOrigins` walks every layer once and records
`origins[node] = layer.Context`. Entries already present are preserved,
which leaves room for follow-ups that pre-stamp extends clones with the
base layer's context.

### 5. Cross-file merge

`mergeLayers` folds the layers left-to-right with `override.MergeNode`.
`ConfigFiles[0]` is the base, every later layer overrides. The combined
`!reset` / `!override` path list is collected; `node.ApplyResetPaths`
applies it to the merged tree once the fold is complete. The `include:`
key is then deleted from the result — it was consumed in phase 2 and
must not appear in the final project.

### 6. Lazy interpolation

`interpolateMerged` walks the merged tree and substitutes `${VAR}` per
scalar using `origins[scalar].Environment` as the lookup. The
interpolation hook also stamps `node.Tag` according to
`tagsForCasts()` so a `published: "80"` declared as a quoted string
decodes as an integer downstream without any explicit cast pass.

### 7. Schema validation

`validateAndStripVersion` runs the JSON-schema validator on a decoded
view of the merged tree before any canonicalization. Catching structural
errors here keeps the canonical transform free of defensive checks.
A successful validation also strips the legacy top-level `version`
attribute and emits the obsolete-version warning per file.

### 8. Per-scalar bare-key environment resolution

`ResolveEnvironmentNode` rewrites every bare `KEY` entry under
`services.*.environment`, `secrets.*.environment` and
`configs.*.environment` to `KEY=value`, picking each `value` from
`origins[scalar].Environment`. Bare keys without a match in scope are
left as-is, matching the v2 behavior that separates "interpolation
produced an empty string" from "no value found".

### 9. Secret / config `environment:` capture

`CaptureSecretConfigContent` returns two `name -> resolved value` maps
recorded *before* CanonicalNode reshuffles pointers. The maps are
applied at the very end of the pipeline (`ApplySecretConfigContent`)
after validation, so the synthesized `content:` does not trip the
content/environment mutual-exclusivity rule.

### 10. Path resolution (pre-canonical)

When `ResolvePaths` is on, `paths.ResolveRelativePathsNode` walks the
tree with a `WorkingDirFor` closure that picks each scalar's WorkingDir
from the origins map. Each handler in `paths/node.go` is path-pattern
keyed and operates on its specific shape (`absScalar`, `absVolumeMount`,
`absEnvFileShortForm`, ...). Paths that look already absolute (Unix or
Windows) are skipped; relative paths are joined against the right WD.

### 11. Canonicalization

`buildServiceContexts` snapshots, *before* `transform.CanonicalNode`
re-encodes the tree, a `service name -> WorkingDir` map. The canonical
transform converts every short form to its long form by going through a
`map[string]any` round-trip; node pointers are not stable across it, so
this snapshot is how the next two phases keep per-service attribution.

### 12. Defaults + post-canonical path resolution

`setDefaultValuesNode` runs the v2 `transform.SetDefaultValues` through
a temporary map projection. `resolveDefaultBuildContext` then walks the
default-`.` build contexts and rewrites them with the service's
WorkingDir from the snapshot. `resolveServiceVolumeSources` does the
same for short-form bind volumes whose source still carries the leading
`.`/`..` indicator — they were skipped pre-canonical because
`absVolumeMount` only matches the long form.

### 13. Compose-rule validation

`validation.ValidateNode` runs the cross-cutting rules (volumes
referenced by services exist, secrets / configs declare exactly one
source, network drivers and IPAM consistency, ...). The project name
is required to be non-empty at this point unless `SkipValidation` is
set.

### 14. Normalization

`NormalizeNode` applies the canonical defaults (default network, implicit
`depends_on` derived from `network_mode`, models containing files, ...).
The current implementation reuses the v2 `Normalize` through a
map roundtrip; a Node-native port is on the roadmap.

### 15. Trim + finalize

`omitEmptyNode` drops entries whose value collapsed to empty after
interpolation (`dns: ${UNSET}` produces `dns: ""` which is then
removed). `ApplySecretConfigContent` injects the captured
secret/config `content` scalars. The returned `*yaml.Node` is the
canonical merged tree.

## Projection

The merged tree is fed into one of two helpers:

- **`nodeToProject(root, opts, cd)`** strips the `name` scalar so it
  cannot override `opts.projectName`, `root.Decode(&project)` projects
  the tree into `*types.Project` via the per-type `UnmarshalYAML`
  methods registered on every compose-go type (no mapstructure,
  no map intermediate). It then applies the project-level
  post-decode passes: `decodeKnownExtensions` re-decodes registered
  `x-*` targets, `EnvFileScopes` is stamped from `opts.envFileScopes`,
  `WithProfiles` / `WithSelectedServices` / `WithoutUnnecessaryResources`
  prune the project, `WithServicesEnvironmentResolved` and
  `WithServicesLabelsResolved` finish the environment plumbing.
- **`nodeToModel(root)`** projects the tree into a `map[string]any` via
  a single `root.Decode(&dict)` call. `OmitEmpty` and the
  secret/config environment resolution have already run at the node
  level, so the dict matches the legacy v2 loadYamlModel output.

`Project.EnvFileScopes` is the side-table that ties the two halves
together: `load` records, for every `env_file` path, the environment
of the layer that declared it, and
`WithServicesEnvironmentResolved` consults it when interpolating
the file content. This is what lets a `secret` declared inside an
included file see variables introduced by the include's own `env_file`.

## Extension points

- **`ResourceLoader`** (`loader.ResourceLoader`) plugs in custom
  protocols for `include.path` and `extends.file`. The built-in
  `localResourceLoader` is always appended last so every other loader
  has a chance to claim the URI first. Each loader exposes `Accept`,
  `Load` (returns a local path), `Dir` (parent directory rendered
  relative to the project root when possible).
- **`KnownExtensions`** maps `x-foo` to a target type. After decode,
  `decodeKnownExtensions` walks every Extensions map (project, service,
  network, volume, config, secret) and re-decodes registered entries
  into the target type via a yaml round-trip.
- **`Listeners`** receive structured events emitted by the pipeline
  (`extends`, `include`, ...). They are append-only metadata and do
  not influence the merge result.

## File map

```
loader/
  load.go                # orchestrator (load), pipeline glue
  load_layer.go          # parse + reset + alias normalization
  load_include.go        # CollectIncludeLayers, env_file scope plumbing
  load_extends.go        # ApplyExtendsToLayer, chained extends
  loader.go              # Options, ResourceLoader, public entry points,
                         # nodeToProject / nodeToModel projections
  normalize_node.go      # NormalizeNode bridge
  resolve_environment_node.go  # bare-key + secret/config env resolution
  reset.go               # !reset / !override processor
  interpolate.go         # interpolateMerged
  transform/             # CanonicalNode + per-rule short -> long form
  internal/node/         # Layer, SourceContext, walker, merge primitives,
                         # alias normalization
override/                # MergeNode + EnforceUnicityNode + per-path rules
paths/                   # ResolveRelativePathsNode + per-path resolvers
interpolation/           # Per-scalar substitution engine
validation/              # ValidateNode + per-rule checks
schema/                  # JSON Schema definitions + validator
types/                   # Project, ServiceConfig, ..., UnmarshalYAML
                         # methods (yaml.v4 native, no mapstructure)
```

## Adding a new transform / validation rule

1. **Decide whether the rule is structural or behavioral.**
   Schema-level checks (top-level kind, required fields, enum values)
   belong in `schema/`. Compose-specific cross-references (a service's
   `volumes_from` actually exists, a network is declared, ...) belong in
   `validation/`.
2. **Pick the right pipeline phase.** Path-shaped values land in `paths/`
   (pre-canonical) or in the post-canonical helpers of `load.go` when
   they need per-service attribution. Short-form / long-form rewrites
   land in `transform/`.
3. **Register a per-path handler.** The walkers in `paths/`,
   `override/` and `validation/` are all keyed by `tree.Path` patterns;
   add an entry and a function.
4. **Add a fixture.** Each transformer, resolver and validator has a
   matching fixture under `loader/testdata/`; add one that covers the
   new rule, declare its expected `*Project` in a `*_test.go` and run
   it through `LoadWithContext`.
