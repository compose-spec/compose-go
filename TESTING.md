# Testing compose-go: the contract

compose-go is the reference implementation of the [Compose
Specification](https://github.com/compose-spec/compose-spec). Its tests are
the executable counterpart of the spec: the document humans read to know what
behavior is locked, and the surface coding agents work against. A test must
state *what clause of the spec it locks* and express its expectation in the
most legible language available — ideally compose YAML itself, not Go
internals.

## Official patterns

Two patterns have proven themselves in this codebase; new tests should use
one of them.

### Attribute conformance tests — `loader/tests/`

One file per compose attribute, named after it (`pull_policy_test.go`,
`extra_hosts_test.go`). Each file:

- opens with a comment linking the spec section it locks
  (`https://github.com/compose-spec/compose-spec/blob/main/05-services.md#<attribute>`)
  and quoting the normative sentence being exercised;
- loads inline YAML via the `load(t, yaml)` helper and asserts on the result;
- re-runs its expectations after `roundTrip(t, p)` (YAML and JSON
  marshal/reload), so the attribute is locked end to end.

The completeness test (`loader/tests/completeness_test.go`) walks the JSON
schema and fails when a service attribute has no declared test file: adding a
property to the schema forces either a conformance test or an explicit,
reviewed entry in the known-gaps list.

### YAML-in / YAML-out — `override/`, and `loadsAs` in `loader/tests/`

When the behavior under test is a document transformation (merge, override,
canonicalization of short syntaxes), express the expectation as literal YAML,
not as Go struct literals:

- `override/`: every test is one `assertMergeYaml(t, base, override, want)`
  call — three YAML blocks, nothing else.
- `loader/tests/`: `loadsAs(t, input, canonical)` asserts that the input
  loads into the canonical model *expressed as YAML* (the project is loaded,
  marshaled back, and compared as YAML trees). Prefer it over field-by-field
  assertions whenever the interesting behavior is how a syntax is parsed and
  canonicalized; keep typed assertions for behaviors the canonical form does
  not surface.

The point of both: the expected value reads as compose documentation, is
diffable in review, and requires no knowledge of `types.Project` internals.

## Rules

- **State the intent.** Every test (or file, for single-attribute files)
  carries a comment saying what invariant it locks — phrased as an
  obligation, with a link to the spec section, and to the issue for
  regression tests. A reviewer must never have to reverse-engineer *why* a
  value is expected.
- **Tables name their cases and run them.** A table with a `name`/`doc`
  field must execute each row inside `t.Run(name, ...)`; a case label that
  does not appear in failure output is dead weight.
- **Error assertions check the config path and the message.** Use
  `assert.ErrorContains` with the offending config path
  (`services.foo.ports`) plus the human-readable fragment. Do not assert
  full exact error strings unless the message itself is the contract
  (`validation/` does this deliberately).
- **Know what the helpers skip.** `load` / `loadYAML` set
  `SkipNormalization` and `SkipConsistencyCheck`: they test the
  parse/merge/interpolate stages only. A behavior involving normalization or
  consistency checks needs an explicit `Load` call with those stages enabled
  — do not assume the helper covered them.
- **A new check or helper is generic and named after what it observes**, and
  lives next to the existing ones (`loader/tests/helpers_test.go`,
  `override/merge_test.go`) where the whole vocabulary can be reviewed as
  one file.

## Running

```sh
make test              # everything
go test ./loader/...   # one package
go test ./loader/tests/ -run TestExtraHosts -v
```
