/*
   Copyright 2022 The Compose Specification Authors.

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

package utils

import (
	"runtime"
	"strings"
)

var (
	// isCaseInsensitiveEnvVars is true on platforms where environment variable names are treated case-insensitively.
	isCaseInsensitiveEnvVars = (runtime.GOOS == "windows")
)

// EnvResolver returns resolver for environment variables suitable for the current platform.
// Expected to be used with `MappingWithEquals.Resolve`.
// Updates in environments may not be reflected.
func EnvResolver(environments map[string]string) func(string) (string, bool) {
	return EnvResolverWithCase(environments, isCaseInsensitiveEnvVars)
}

// EnvResolverWithCase returns resolver for environment variables with the specified case-sensitive condition.
// Expected to be used with `MappingWithEquals.Resolve`.
// Updates in environments may not be reflected.
func EnvResolverWithCase(environment map[string]string, caseInsensitive bool) func(string) (string, bool) {
	if environment == nil {
		return func(s string) (string, bool) {
			return "", false
		}
	}
	if !caseInsensitive {
		return func(s string) (string, bool) {
			v, ok := environment[s]
			return v, ok
		}
	}
	// variable names must be treated case-insensitively.
	// Resolves in this way:
	// * Return the value if its name matches with the passed name case-sensitively.
	// * Otherwise, return the value if its lower-cased name matches lower-cased passed name.
	//     * The value is indefinite if multiple variable matches.
	var loweredEnvironment map[string]string
	return func(s string) (string, bool) {
		v, ok := environment[s]
		if ok {
			return v, ok
		}
		if loweredEnvironment == nil {
			// create only when it's necessary
			newLoweredEnvironment := make(map[string]string, len(environment))
			for k, v := range environment {
				newLoweredEnvironment[strings.ToLower(k)] = v
			}
			loweredEnvironment = newLoweredEnvironment
		}
		v, ok = loweredEnvironment[strings.ToLower(s)]
		return v, ok
	}
}
