# Migrating from compose-go v2 to v3

v3 is a major release. The public entry points (`LoadWithContext`,
`LoadModelWithContext`, `Options`, `*types.Project`) keep the same
signature, so most callers compile against v3 with only the module
path change. This document lists the breaking changes a caller might
trip over and the behavioral changes a caller might rely on.

## Module path

```go
// v2
import "github.com/compose-spec/compose-go/v2/loader"

// v3
import "github.com/compose-spec/compose-go/v3/loader"
```

Update every import path in your project from `v2` to `v3`.

## Removed APIs

| v2 symbol                                | v3 replacement                                                            |
| ---------------------------------------- | ------------------------------------------------------------------------- |
| `loader.Transform(source, target)`       | `(*yaml.Node).Decode(&target)` (yaml.v4 native, no mapstructure)         |
| `loader.ModelToProject(dict, opts, cd)`  | `loader.LoadWithContext(ctx, cd, opts...)` returns `*types.Project`      |
| `loader.ApplyInclude(...)`               | Internal -- includes are processed by `loader.CollectIncludeLayers`      |
| `loader.ApplyExtends(...)`               | Internal -- `loader.ApplyExtendsToLayer` on the yaml.Node tree           |
| `loader.Normalize(dict, env)`            | Still works as a wrapper, prefer `loader.NormalizeNode(*yaml.Node, env)` |
| `loader.OmitEmpty(dict)`                 | Internal -- runs as part of `Load`                                       |
| `loader.ResolveEnvironment(dict, env)`   | Internal -- per-scalar `loader.ResolveEnvironmentNode`                   |
| `types.<T>.DecodeMapstructure(value)`    | Replaced by `UnmarshalYAML(value *yaml.Node) error` on every type        |

The `Transform` removal is the most visible break: code that turned an
arbitrary `map[string]any` into a typed compose-go struct via
`loader.Transform` should call `yaml.Marshal` + `yaml.Unmarshal` (or
`(*yaml.Node).Decode`) instead.

## Removed dependency

`github.com/go-viper/mapstructure/v2` is no longer in `go.mod`. Every
compose-go type now exposes `UnmarshalYAML(*yaml.Node) error` and the
loader projects directly into `*types.Project` via yaml.v4. A
downstream module that imported mapstructure only because compose-go
required it can drop it too.

## YAML tags

Two structs gained explicit `yaml:` tags on fields that previously
relied on yaml.v3's lowercased-field-name fallback. The serialized
form is unchanged; the tags are required because yaml.v4 is stricter:

```go
type WeightDevice struct {
    Path   string `yaml:"path,omitempty"`
    Weight uint16 `yaml:"weight,omitempty"`
    // ...
}

type ThrottleDevice struct {
    Path string    `yaml:"path,omitempty"`
    Rate UnitBytes `yaml:"rate,omitempty"`
    // ...
}
```

Downstream forks that embed these structs in their own types should
mirror the tags.

## Error types

Most user-facing failures now surface as `*errdefs.Diagnostic`. The
type carries the source file, line, column and dotted compose path
alongside the underlying cause:

```go
type Diagnostic struct {
    File   string
    Line   int
    Column int
    Path   string
    Cause  error
}
```

`Diagnostic.Error()` renders as `file:line:col: path: cause` (each
segment elided when missing). Existing error handling that matched on
the legacy string format needs to switch to substring matching or use
`errors.As` to inspect the typed value:

```go
var diag *errdefs.Diagnostic
if errors.As(err, &diag) {
    fmt.Printf("at %s:%d:%d: %s\n", diag.File, diag.Line, diag.Column, diag.Cause)
}
```

`errors.Is` and `errors.As` still walk through the wrapped Cause.

The following sites are now wrapped:

- JSON Schema validation (`schema.Validate` failures)
- Compose-rule validation (`validation.ValidateNode` failures)
- Interpolation strict mode (`${VAR:?msg}` against unset variables)
- Include cycle (`include cycle detected`)
- `include must be a list` and per-entry shape errors
- `extends.NAME.service is required` and related extends ref errors
- `services.NAME must be a mapping`
- `cannot extend service in F: no services section`
- `cannot extend service in F: service Q not found`

Tests that asserted on the exact legacy strings need to update to the
new prefixed form, e.g.

```diff
- assert.Error(t, err, "extends.test.service is required")
+ assert.Error(t, err, "(inline):7:7: services.test.extends: extends.test.service is required")
```

A typed assertion via `errors.As(err, &diag)` is more robust.

## Behavioral changes

### Lazy, per-scalar interpolation

`${VAR}` is now substituted per scalar in the merged tree, with the
lookup scoped to the `SourceContext.Environment` of the layer that
declared that scalar. The classic v2 behavior interpolated each file
in its own scope at parse time; the result for a flat project is
unchanged, but two scenarios behave differently:

- A variable declared by an include's `env_file` is now visible to
  scalars declared inside that include, including the content of
  service-level `env_file` entries declared in the included file.
- A variable from the parent shell environment is still visible to
  every scalar (parent-wins via `Mapping.Merge`).

See `docs/Interpolation.md` for the full scope composition rules.

### Per-include path resolution

Relative paths inside an included file are resolved against the
include's own `project_directory`, not the project root. v2 always
joined relative paths with the project root, which was a known
limitation.

### `Project.EnvFileScopes`

The `*types.Project` returned by `LoadWithContext` now carries an
`EnvFileScopes map[string]Mapping` side-table keyed by absolute
env_file path. `WithServicesEnvironmentResolved` consults it when
interpolating env_file content so a file referenced from an include
block resolves variables against the include env_file values rather
than only the project-wide environment. The map is hidden from
`yaml.Marshal` / `json.Marshal` (`yaml:"-" json:"-"`) and preserved
by `deepCopy`.

### `Project.Sources` (planned)

Not yet exposed. The internal `buildPathPositions` snapshot already
records `path -> (file, line, column)` for diagnostics; a follow-up
will attach it to `*Project.Sources` behind an `Options.Diagnostics`
opt-in for tooling that wants to surface source locations in their UI.

### `dns: ${UNSET}` collapsing

The legacy "empty dns drops the list" behavior was a map-level
`OmitEmpty` pass. It now runs on the yaml.Node tree at the same
position in the pipeline. A `dns: ${UNSET}` that interpolates to an
empty string still drops the entry; the decoded `Project.Services.X.DNS`
remains an empty slice.

### FileMode parsing

`type FileMode` accepts the same set of source forms (`"0440"`,
`0440`, `288`, `"288"`), but the precedence order changed: octal is
tried first, then decimal as fallback. The motivation is the YAML
round-trip done by extends / canonical, which can re-emit an octal
literal as its decimal equivalent.

## New helpers

A handful of building blocks landed in v3 that didn't exist in v2:

- `internal/node.Layer`, `internal/node.SourceContext`,
  `internal/node.NormalizeAliases`, `internal/node.ResolveResetOverride`
  -- the yaml.Node-centric primitives. Unexported package
  (`internal`), not for public use.
- `loader.LoadLayer`, `loader.CollectIncludeLayers`,
  `loader.ApplyExtendsToLayer` -- per-phase entry points if you need
  to drive the loader in pieces.
- `loader.ResolveEnvironmentNode`, `loader.CaptureSecretConfigContent`,
  `loader.ApplySecretConfigContent` -- node-level passes called by the
  orchestrator. Exported because they can be useful in custom
  pipelines.

For a tour of how everything fits together, see `docs/Architecture.md`.

## Common upgrade recipes

### "My code called `loader.Transform`"

```diff
- err := loader.Transform(source, &target)
+ buf, err := yaml.Marshal(source)
+ if err != nil { return err }
+ err = yaml.Unmarshal(buf, &target)
```

Or, when `source` is already a `*yaml.Node`:

```diff
- err := loader.Transform(asMap, &target)
+ err := node.Decode(&target)
```

### "My code called `loader.ModelToProject`"

Switch to the public `LoadWithContext`:

```diff
- dict, err := loader.LoadModelWithContext(ctx, cd, opts...)
- if err != nil { return nil, err }
- project, err := loader.ModelToProject(dict, optsStruct, cd)
+ project, err := loader.LoadWithContext(ctx, cd, opts...)
```

`Options` is built and threaded internally; pass the same option
functions you already pass to `LoadModelWithContext`.

### "My tests asserted on `validating X: ...` strings"

The new format is `file:line:col: path: cause`. Either widen the
match to `strings.Contains` on the cause, or assert against the
typed `*errdefs.Diagnostic`:

```go
var diag *errdefs.Diagnostic
if assert.Check(t, errors.As(err, &diag)) {
    assert.Equal(t, diag.Path, "services.web.image")
    assert.Assert(t, strings.Contains(diag.Cause.Error(), "must be a string"))
}
```

### "My type embedded `WeightDevice` / `ThrottleDevice`"

Add the explicit `yaml:` tags on the fields:

```go
Path   string    `yaml:"path,omitempty" json:"path,omitempty"`
Weight uint16    `yaml:"weight,omitempty" json:"weight,omitempty"`
// or, for ThrottleDevice
Rate   UnitBytes `yaml:"rate,omitempty" json:"rate,omitempty"`
```
