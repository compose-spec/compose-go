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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/compose-spec/compose-go/v2/cli"
	"go.yaml.in/yaml/v4"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println(`
Validates a compose file conforms to the Compose Specification

Usage: compose-spec [OPTIONS] COMPOSE_FILE [COMPOSE_OVERRIDE_FILE]`)
	}

	var skipInterpolation, skipResolvePaths, skipNormalization, skipConsistencyCheck bool
	var format string

	flag.BoolVar(&skipInterpolation, "no-interpolation", false, "Don't interpolate environment variables.")
	flag.BoolVar(&skipResolvePaths, "no-path-resolution", false, "Don't resolve file paths.")
	flag.BoolVar(&skipNormalization, "no-normalization", false, "Don't normalize compose model.")
	flag.BoolVar(&skipConsistencyCheck, "no-consistency", false, "Don't check model consistency.")
	flag.StringVar(&format, "format", "yaml", "Output format (yaml|json).")
	flag.Parse()

	wd, err := os.Getwd()
	if err != nil {
		exitError("can't determine current directory", err)
	}

	options, err := cli.NewProjectOptions(flag.Args(),
		cli.WithWorkingDirectory(wd),
		cli.WithOsEnv,
		cli.WithDotEnv,
		cli.WithConfigFileEnv,
		cli.WithDefaultConfigPath,
		cli.WithInterpolation(!skipInterpolation),
		cli.WithResolvedPaths(!skipResolvePaths),
		cli.WithNormalization(!skipNormalization),
		cli.WithConsistency(!skipConsistencyCheck),
	)
	if err != nil {
		exitError("failed to configure project options", err)
	}

	model, err := options.LoadModel(context.Background())
	if err != nil {
		exitError("failed to load project", err)
	}

	var raw []byte
	switch format {
	case "yaml":
		raw, err = yaml.Marshal(model)
		if err != nil {
			exitError("failed to marshall project", err)
		}
	case "json":
		raw, err = json.MarshalIndent(model, "", "  ")
		if err != nil {
			exitError("failed to marshall project", err)
		}
	default:
		_ = fmt.Errorf("unsupported output format %s", format)
		os.Exit(1)
	}

	fmt.Println(string(raw))
}

func exitError(message string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v", message, err)
	os.Exit(1)
}
