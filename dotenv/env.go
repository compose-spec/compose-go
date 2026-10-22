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

package dotenv

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compose-spec/compose-go/v2/logging"
)

func GetEnvFromFile(currentEnv map[string]string, filenames []string) (map[string]string, error) {
	return GetEnvFromFileWithLogger(currentEnv, filenames, nil)
}

// GetEnvFromFileWithLogger is GetEnvFromFile sending the warnings raised while
// parsing the files (a variable that is not set, referenced by a file) to l
// instead of the standard logrus logger, which receives them when l is nil.
func GetEnvFromFileWithLogger(currentEnv map[string]string, filenames []string, l logging.Logger) (map[string]string, error) {
	envMap := make(map[string]string)

	for _, dotEnvFile := range filenames {
		abs, err := filepath.Abs(dotEnvFile)
		if err != nil {
			return envMap, err
		}
		dotEnvFile = abs

		s, err := os.Stat(dotEnvFile)
		if os.IsNotExist(err) {
			return envMap, fmt.Errorf("couldn't find env file: %s", dotEnvFile)
		}
		if err != nil {
			return envMap, err
		}

		if s.IsDir() {
			if len(filenames) == 0 {
				return envMap, nil
			}
			return envMap, fmt.Errorf("%s is a directory", dotEnvFile)
		}

		b, err := os.ReadFile(dotEnvFile)
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("couldn't read env file: %s", dotEnvFile)
		}
		if err != nil {
			return envMap, err
		}

		err = parseWithLogger(bytes.NewReader(b), envMap, func(k string) (string, bool) {
			v, ok := currentEnv[k]
			if ok {
				return v, true
			}
			v, ok = envMap[k]
			return v, ok
		}, l)
		if err != nil {
			return envMap, fmt.Errorf("failed to read %s: %w", dotEnvFile, err)
		}
	}

	return envMap, nil
}
