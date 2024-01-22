# compose-go
[![Continuous integration](https://github.com/compose-spec/compose-go/actions/workflows/ci.yml/badge.svg)](https://github.com/compose-spec/compose-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/compose-spec/compose-go.svg)](https://pkg.go.dev/github.com/compose-spec/compose-go)

Go reference library for parsing and loading Compose files as specified by the
[Compose specification](https://github.com/compose-spec/compose-spec).

## Usage

```go
package main

import (
	"fmt"
	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"log"
)

func main() {
	composeFilePath := "docker-compose.yml"
	projectName := "my_project"

	options, err := loader.NewProjectOptions(
		[]string{composeFilePath},
		types.WithOsEnv,
		types.WithDotEnv,
		types.WithName(projectName),
	)
	if err != nil {
		log.Fatal(err)
	}

	project, err := loader.ProjectFromOptions(options)
	if err != nil {
		log.Fatal(err)
	}

	// Use the MarshalYAML method to get YAML representation
	projectYAML, err := project.MarshalYAML()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(projectYAML))
}

```

## Build the library

To build the library, you could either use the makefile
```bash
make build
```
or use the go build command
```bash
go build ./...
```

## Run the tests
You can run the tests with the makefile
```bash
make test
```
or with the go test command
```bash
gotestsum ./...
```

## Other helpful make commands
Run the linter
```bash
make lint
```

Check the license headers
```bash
make check_license
```

Check the `compose-spec.json` file is sync with the `compose-spec` repository
```bash
make check_compose_spec
```

## Used by

* [compose](https://github.com/docker/compose)
* [containerd/nerdctl](https://github.com/containerd/nerdctl)
* [compose-cli](https://github.com/docker/compose-cli)
* [tilt.dev](https://github.com/tilt-dev/tilt)
* [kompose](https://github.com/kubernetes/kompose)
* [kurtosis](https://github.com/kurtosis-tech/kurtosis/)
* [testcontainers-go's Compose module](https://github.com/testcontainers/testcontainers-go/tree/main/modules/compose)
