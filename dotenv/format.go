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
	"fmt"
	"io"

	"github.com/compose-spec/compose-go/v2/logging"
)

const DotEnv = ".env"

// format is a registered env_file format: either the built-in .env one, which
// reports its warnings to a logger (parseWithLogger), or one added by
// RegisterFormat, which has no way to be given one (parse).
type format struct {
	parse           Parser
	parseWithLogger func(r io.Reader, filename string, vars map[string]string, lookup func(key string) (string, bool), l logging.Logger) error
}

var formats = map[string]format{
	DotEnv: {parseWithLogger: parseDotEnv},
}

func parseDotEnv(r io.Reader, filename string, vars map[string]string, lookup func(key string) (string, bool), l logging.Logger) error {
	err := parseWithLogger(r, vars, lookup, l)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}
	return nil
}

type Parser func(r io.Reader, filename string, vars map[string]string, lookup func(key string) (string, bool)) error

func RegisterFormat(name string, p Parser) {
	formats[name] = format{parse: p}
}

func ParseWithFormat(r io.Reader, filename string, vars map[string]string, resolve LookupFn, f string) error {
	return ParseWithFormatAndLogger(r, filename, vars, resolve, f, nil)
}

// ParseWithFormatAndLogger is ParseWithFormat sending the warnings raised while
// parsing (a variable that is not set, referenced by the file) to l instead of
// the standard logrus logger, which receives them when l is nil. A format added
// by RegisterFormat is not given the logger: it reports as it chooses.
func ParseWithFormatAndLogger(r io.Reader, filename string, vars map[string]string, resolve LookupFn, f string, l logging.Logger) error {
	if f == "" {
		f = DotEnv
	}
	fn, ok := formats[f]
	if !ok {
		return fmt.Errorf("unsupported env_file format %q", f)
	}
	if fn.parseWithLogger != nil {
		return fn.parseWithLogger(r, filename, vars, resolve, l)
	}
	return fn.parse(r, filename, vars, resolve)
}
