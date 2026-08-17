# compose-go — agent instructions

compose-go is the reference implementation of the [Compose
Specification](https://github.com/compose-spec/compose-spec), consumed by
Docker Compose and other Compose implementations.

## Build & Test

- Test all: `make test` (or `go test ./...`)
- Test one package: `go test ./loader/...`
- Lint: `golangci-lint run` (config in `.golangci.yml`)

## Tests are the contract

Read `TESTING.md` before writing or modifying any test. In short: tests are
the executable counterpart of the spec — one conformance file per attribute
in `loader/tests/` linking the spec section it locks, YAML-in/YAML-out
expectations (`assertMergeYaml`, `loadsAs`) whenever the behavior is a
document transformation, intent stated as a comment on every test, and
`t.Run` for every named table case. The completeness test in
`loader/tests/completeness_test.go` will fail if a schema attribute is added
without a declared test.

## Git

- Every commit must carry a DCO `Signed-off-by` trailer (`git commit -s`).
