# Interpolation of YAML values

This document describes how `${VAR}` expressions are substituted into
compose files, which scope each scalar is interpolated against, and how
`include` with `env_file` changes that scope.

## What gets interpolated

Interpolation operates on **every scalar in the merged tree** after
includes and extends have been folded in (steps 5–6 of the pipeline,
see `docs/Architecture.md`). It happens in place on the `*yaml.Node`
tree, so the substitution preserves the original Style (single-quoted,
double-quoted, plain, ...). Type casting is applied at the same time by
rewriting `Node.Tag` (see "Type casts" below).

The syntax is a strict subset of Bash parameter expansion (full list in
`template/`):

| Form                  | Behavior                                                           |
| --------------------- | ------------------------------------------------------------------ |
| `${VAR}` / `$VAR`     | Substitute the value, fail in strict mode if unset.                |
| `${VAR-default}`      | Use `default` when `VAR` is unset.                                 |
| `${VAR:-default}`     | Use `default` when `VAR` is unset *or* empty.                      |
| `${VAR?error}`        | Fail with `error` when `VAR` is unset.                             |
| `${VAR:?error}`       | Fail with `error` when `VAR` is unset *or* empty.                  |
| `${VAR+value}`        | Use `value` when `VAR` is set (even to "").                        |
| `${VAR:+value}`       | Use `value` when `VAR` is set and non-empty.                       |
| `$$` / `${$}`         | Literal `$`, no substitution.                                      |

Bracket bodies may nest (`${OUTER:-${INNER:-fallback}}`) and the
default / error operands themselves go through substitution.

## The "lazy" principle

The classic Compose loader interpolated each file in its own scope **at
parse time**, before merge. `compose-go` v3 merges first and
interpolates last, but it does so **per scalar** rather than at the
top of the tree. Every scalar carries an implicit "where did this come
from" via the `origins` side-table, and the substitution engine picks
the lookup function for that scalar from the layer's
`SourceContext.Environment`:

```go
interp.Options{
    LookupValue: func(node *yaml.Node, key string) (string, bool) {
        ctx := origins[node]
        if ctx == nil {
            return "", false
        }
        v, ok := ctx.Environment[key]
        return v, ok
    },
}
```

This is the **lazy interpolation principle**: a value introduced by one
layer's environment is only visible to the scalars that came from that
layer. Two consequences fall out of it:

- A variable declared in an include's `env_file` is visible to scalars
  declared inside that include (and to scalars from files that include
  declares in turn), but never to the parent that did the include.
- A variable declared in the top-level shell environment is visible to
  every scalar **unless** an include layer happens to redefine it; in
  that case the include's own value wins inside the include scope.

## Composing the environment of a layer

`SourceContext.Environment` is built layer by layer. Going down the
chain:

1. **Root context.** `cd.Environment` (the value the caller passes to
   `LoadWithContext`) seeds the root layer.
2. **`COMPOSE_PROJECT_NAME`.** `projectName(cd, opts)` stamps the
   resolved project name onto `cd.Environment` before any layer is
   parsed, so it is visible to every scalar in every file.
3. **`include.env_file`.** When an entry under the top-level `include:`
   block carries an `env_file:` (relative paths are resolved against
   the parent WorkingDir), `resolveIncludeEnvironment` loads each file
   in declaration order on top of the parent environment via
   `Mapping.Merge`. **Existing keys win**: a key already present in the
   parent context keeps its parent value, the include's `env_file`
   value is dropped. This matches the v2 behavior where the shell
   environment overrides `.env` entries.
4. **Implicit `<project_directory>/.env`.** If an include declares no
   explicit `env_file:` *and* a `.env` file exists at the include's
   `project_directory`, it is loaded as if it had been listed
   explicitly. A single `/dev/null` entry in `env_file:` disables this
   implicit lookup *and* skips the listed entry.
5. **Extends.** Layers loaded through `extends.file` inherit the parent
   layer's environment as-is. There is no `env_file` mechanism on the
   `extends` block itself.

The resulting `SourceContext.Environment` is the lookup table used by
every scalar that originates from that layer.

## Worked example: include scope

Consider this fixture (close to `testdata/include/env_file/`):

```yaml
# compose.yaml
include:
  - path: sub/compose.yaml
    env_file:
      - sub/.env.include

services:
  parent:
    image: ${IMG:-base}
```

```yaml
# sub/compose.yaml
services:
  app:
    image: alpine
    env_file:
      - extra.env
```

```sh
# sub/.env.include
BAR=bar
IMG=ignored-because-shell-wins
```

```sh
# sub/extra.env
FOO=$BAR
OVR=${BAR:-fallback}
```

Loaded with `cd.Environment = {"IMG": "shell"}`:

| Scalar                                    | Layer                  | Lookup observes                         | Result                       |
| ----------------------------------------- | ---------------------- | --------------------------------------- | ---------------------------- |
| `services.parent.image: ${IMG:-base}`     | top-level              | shell `IMG=shell`                       | `image: shell`               |
| `services.app.image: alpine`              | sub include            | shell + include `env_file`              | `image: alpine` (literal)    |
| `extra.env` `FOO=$BAR`                    | sub include `env_file` | shell + include `env_file` (`BAR=bar`)  | `FOO=bar`                    |
| `extra.env` `OVR=${BAR:-fallback}`        | sub include `env_file` | shell + include `env_file` (`BAR=bar`)  | `OVR=bar`                    |

Three things to note:

- `IMG=shell` from the caller's environment wins over the include
  `env_file` value because parent-wins is enforced by `Mapping.Merge`.
- `BAR` is **not** visible to the top-level `parent` service even
  though it lives in the same project — the scope of `BAR` is the
  include layer.
- `extra.env` is itself processed in the include's scope: its content
  is interpolated against the **include's `env_file`**, not against the
  caller's shell environment, so `$BAR` resolves to `bar`.

## `env_file` on a service (vs on `include`)

`services.*.env_file` is a different code path from `include.env_file`,
but the same lazy principle applies. `WithServicesEnvironmentResolved`
(see `types/project.go`) reads each file, interpolates `${VAR}` inside
the file content, then merges the result into the service's
`Environment` map. The lookup function it passes prefers, in order:

1. The service's already-resolved `Environment` (so a variable set on
   the service can be referenced by a later `env_file` entry).
2. `Project.EnvFileScopes[envFile.Path]` — the *layer* environment
   captured at `env_file` declaration time. This is what makes the
   scope honor lazy interpolation across includes.
3. The project-wide `Environment` (fallback when the entry was
   declared at the top level and no scope was captured).

`Project.EnvFileScopes` is populated during `load`: when a layer
declares an `env_file` entry, the loader records
`scopes[absoluteEnvFilePath] = layer.Context.Environment`. The
resulting map is attached to the `*Project` by `nodeToProject` and
preserved across `deepCopy`. From the consumer side, calling
`WithServicesEnvironmentResolved` on a project built with an include
that brought its own `env_file` produces the resolved values that
match what an interpolation pass *inside* that include would produce.

## `secrets:` / `configs:` declared with `environment:`

When a `secrets.NAME` or `configs.NAME` entry has the
`environment: VARNAME` shorthand, the value of `VARNAME` is looked up
in the layer that declared the secret/config — exactly the same scope
the surrounding scalar interpolation would use. The lookup happens
during `load` via `CaptureSecretConfigContent`, which walks the tree
pre-canonical (where origin pointers are still valid) and records
`name -> resolved value`. The map is applied to the post-canonical
tree by `ApplySecretConfigContent` so the synthesized `content:`
scalar reaches `SecretConfig.UnmarshalYAML` / `ConfigObjConfig.UnmarshalYAML`.

Practical effect: a secret declared inside an included file can pull
its value from a variable introduced by the include's own
`env_file`. The fixture under `testdata/include/secret_env/`
covers this scenario.

## Type casts

Schema-driven type conversions are wired in at the interpolation step
rather than as a separate post-pass. `tagsForCasts()` maps a
`tree.Path` pattern to a YAML tag (`!!int`, `!!bool`, `!!float`), and
the interpolation hook rewrites `node.Tag` accordingly after
substitution. yaml.v4 then performs the conversion natively at decode
time. The two consequences:

- `published: "${PORT}"` with `PORT=80` decodes as an integer because
  `services.*.ports.*.published` is tagged `!!int`.
- `init: "${INIT}"` with `INIT=true` decodes as a boolean for the same
  reason on `services.*.init`.

The list of cast targets matches the v2 `interpolateTypeCastMapping`;
adding a new one is a one-line entry in `tagsForCasts()`.

## Strict vs lenient mode

`Options.Interpolate.Substitute` is the entry point of the substitution
engine; `cd.LookupEnv` is the default `LookupValue`. The default mode
treats an unset variable as the empty string and emits a warning. To
fail fast on missing variables, use the strict variants in the source
file:

```sh
image: nginx:${TAG:?TAG must be set}
```

The error surfaces at the scalar where the unset variable was
referenced, with the file / line / column of that scalar.

## Gotchas

- **Double dollar.** A literal `$` inside a value must be escaped as
  `$$`, otherwise the engine tries to interpret it. In particular
  `command: "echo $$PATH"` produces `echo $PATH` at runtime.
- **Quoted scalars.** Interpolation operates on the parsed scalar
  value, not on the YAML source. `image: "${TAG}"` and `image: ${TAG}`
  produce the same result (modulo type casts: only the unquoted form is
  eligible for a `!!int` rewrite, the quoted form stays `!!str`).
- **Include order matters.** The include layers are merged in
  declaration order; a later include that ships an
  `env_file:` value for a key already present in an earlier include
  scope does *not* override the earlier scope. Each include keeps its
  own environment composition.
- **Implicit `.env`.** The `.env` lookup at the project root is
  performed by the *caller* (the CLI), not by `compose-go`. The
  loader honors `cd.Environment` as the single source of truth for
  the root context.
