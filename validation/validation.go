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

package validation

import (
	"github.com/compose-spec/compose-go/v2/tree"
)

type checkerFunc func(value any, p tree.Path) error

var checks = map[tree.Path]checkerFunc{
	"volumes.*": checkVolume,
}

func Validate(dict map[string]any) error {
	return check(dict, tree.NewPath())
}

func check(value any, p tree.Path) error {
	for pattern, fn := range checks {
		if p.Matches(pattern) {
			return fn(value, p)
		}
	}
	switch v := value.(type) {
	case map[string]any:
		for k, v := range v {
			err := check(v, p.Next(k))
			if err != nil {
				return err
			}
		}
	case []any:
		for _, e := range v {
			err := check(e, p.Next("[]"))
			if err != nil {
				return err
			}
		}
	}
	return nil
}
