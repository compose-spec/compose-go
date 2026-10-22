# Maintenance

The compose-go library implements the [Compose specification](https://github.com/compose-spec/compose-spec) and is
where the JSON schema of the specification is maintained: `schema/compose-spec.json` in this repository is the
reference, and the copy in the compose-spec repository is synchronized from it.

## Adding or changing an attribute

When the specification gains an attribute (or changes one), the change typically
touches the places below, in this order. `git show --stat ddbe099` (the `umask`
attribute) shows what a real change looks like.

1. **JSON schema**: update `schema/compose-spec.json`. This file is the
   reference: a schema change is validated against the code and its tests here
   before the specification is updated. Do not edit the copy in the compose-spec
   repository: a check there rejects changes that do not come from a bot, and a
   scheduled job of that repository (`.github/workflows/sync.yml`, daily)
   downloads the file from the `main` branch of compose-go and opens a pull
   request. Service attributes live in the `container_spec`, `workload_spec` and
   `service` definitions, the `jobs` element in `job`.
2. **Model**: add the field, with its `yaml` and `json` tags, to the matching
   struct in `types/` (`ContainerSpec`, `WorkloadSpec` or `ServiceConfig` in
   `types/types.go` for a service attribute). Reuse the helper types of the
   package (`StringList`, `Mapping`, `MappingWithEquals`, `HostsList`,
   `Duration`...) when the attribute accepts several syntaxes.
3. **Generated code**: run both generators and commit the result.
   - `make generate` rewrites `types/container_spec_attributes.gen.go`, the list
     of `ContainerSpec` attributes. `make check-generate` is the check run by CI.
   - `make deepcopy` rewrites `types/derived.gen.go` (`goderive`, run in Docker).
4. **Loading pipeline**, only for what the attribute needs:
   - `loader/interpolate.go`: register a cast for a boolean or numeric value, so
     that `${VAR}` interpolation produces the right type;
   - `transform/canonical.go`: expansion of a short syntax into the canonical
     form;
   - `transform/defaults.go`: default values defined by the specification;
   - `override/merge.go`: merge rule, when the default one (scalars are
     overridden, mappings merged, sequences appended) is not the right one, and
     `override/uncity.go` for the uniqueness key of sequence items;
   - `paths/resolve.go`: resolution of a relative path against the project
     directory;
   - `loader/normalize.go`: normalization applied after the merge, e.g. the
     replacement of a legacy attribute by its successor;
   - `validation/` for checks on the raw model, `loader/validate.go` for
     consistency checks on the loaded project: constraints the JSON schema
     cannot express.
5. **Tests**, as described in [TESTING.md](TESTING.md): add a conformance file
   named after the attribute in `loader/tests/` and declare it in `coveredBy` in
   `loader/tests/completeness_test.go`, which reads the schema and fails when a
   service attribute has no declared test file. Add cases in `override/` for a
   merge rule, and in `transform/` for a canonicalization or a default.

Before opening the pull request, run what the CI runs:

```sh
make generate deepcopy      # generated sources must be up to date
make test                   # or go test ./...
make lint                   # golangci-lint, in Docker (or golangci-lint run)
make check-license          # license headers, in Docker
go vet -stdmethods=false ./...
go mod tidy -diff
```

The CI runs the tests on Go 1.24 and 1.25, on Linux, macOS and Windows.

## Releasing

A release is made by pushing a tag on `main`; the
[release workflow](.github/workflows/release.yml) does the rest.

1. **Pick the version.** The module is `github.com/compose-spec/compose-go/v2`,
   so releases stay in the `v2.x.y` range: an incompatible API change is not
   allowed. A new feature bumps the minor version, a fix the patch version. The
   `api compatibility` job of the CI ([ci.yml](.github/workflows/ci.yml)) runs
   [gorelease](https://pkg.go.dev/golang.org/x/exp/cmd/gorelease) on pull
   requests and writes the public API changes since the latest release to the job
   summary. It is informative and never fails the check, but it spots changes
   that are invisible in the diff (embedded structs, promoted fields, changed
   signatures). Run `gorelease` from an up-to-date `main` to review the changes
   of the whole release.
2. **Tag.** List the latest tags with `git tag --sort=-v:refname | head`. Recent
   tags (such as `v2.16.1`) are annotated and signed, with the version as message:

   ```sh
   git tag -s -m vX.Y.Z vX.Y.Z
   git push origin vX.Y.Z
   ```

3. **Release workflow.** A tag matching `v*` first runs the full CI as a gate:
   nothing is published unless it passes. The workflow then builds the binary of
   `cmd/` (`make build`) for linux, darwin and windows on amd64 and arm64,
   generates a `SHA256SUMS` file and creates a **draft** GitHub release with the
   binaries attached and generated release notes.
4. **Publish.** Open the draft in the repository's *Releases* page, review the
   generated notes and publish it.
