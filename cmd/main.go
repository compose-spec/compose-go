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
	"fmt"
	"os"

	"github.com/compose-spec/compose-go/v2/cli"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println(`
Validates a compose file conforms to the Compose Specification

Usage: compose-spec [OPTIONS] COMPOSE_FILE [COMPOSE_OVERRIDE_FILE]`)
	}

	wd, err := os.Getwd()
	if err != nil {
		exitError("can't determine current directory", err)
	}

	options, err := cli.NewProjectOptions(os.Args[1:],
		cli.WithWorkingDirectory(wd),
		cli.WithOsEnv,
		cli.WithDotEnv,
		cli.WithConfigFileEnv,
		cli.WithDefaultConfigPath,
	)
	if err != nil {
		exitError("failed to configure project options", err)
	}

	project, err := cli.ProjectFromOptions(options)
	if err != nil {
		exitError("failed to load project", err)
	}

	yaml, err := project.MarshalYAML()
	if err != nil {
		exitError("failed to marshall project", err)
	}
	fmt.Println(string(yaml))
}

func exitError(message string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v", message, err)
	os.Exit(1)
}
