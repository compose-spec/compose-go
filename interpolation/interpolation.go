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

package interpolation

import (
	"errors"
	"fmt"
	"github.com/compose-spec/compose-go/v2/arraytemplate"
	"os"
	"reflect"

	"github.com/compose-spec/compose-go/v2/template"
	"github.com/compose-spec/compose-go/v2/tree"
)

// Options supported by Interpolate
type Options struct {
	// LookupValue from a key
	LookupValue LookupValue
	// TypeCastMapping maps key paths to functions to cast to a type
	TypeCastMapping map[tree.Path]Cast
	// Substitution function to use
	Substitute func(string, LookupValue) (*SubstitutionResult, error)
}

type SubstitutionResult struct {
	String string
	Array  []string
}

// LookupValue is a function which maps from variable names to values.
// Returns the value as a string and a bool indicating whether
// the value is present, to distinguish between an empty string
// and the absence of a value.
type LookupValue func(key string) (string, bool)

// Cast a value to a new type, or return an error if the value can't be cast
type Cast func(value string) (interface{}, error)

// Interpolate replaces variables in a string with the values from a mapping
func Interpolate(config map[string]interface{}, opts Options) (map[string]interface{}, error) {
	if opts.LookupValue == nil {
		opts.LookupValue = os.LookupEnv
	}
	if opts.TypeCastMapping == nil {
		opts.TypeCastMapping = make(map[tree.Path]Cast)
	}
	if opts.Substitute == nil {
		opts.Substitute = DefaultSubstitute
	}

	out := map[string]interface{}{}

	for key, value := range config {
		interpolatedValue, err := recursiveInterpolate(value, tree.NewPath(key), opts)
		if err != nil {
			return out, err
		}
		out[key] = interpolatedValue
	}

	return out, nil
}

func DefaultSubstitute(t string, lookup LookupValue) (*SubstitutionResult, error) {
	if (arraytemplate.ArraySubstitutionPattern).MatchString(t) {
		arr, err := arraytemplate.Substitute(t, arraytemplate.Mapping(lookup))
		return &SubstitutionResult{Array: arr}, err
	}
	str, err := template.Substitute(t, template.Mapping(lookup))
	return &SubstitutionResult{String: str}, err
}

func recursiveInterpolate(value interface{}, path tree.Path, opts Options) (interface{}, error) {
	switch value := value.(type) {
	case string:
		result, err := opts.Substitute(value, opts.LookupValue)
		if err != nil {
			return value, newPathError(path, err)
		}
		if result.Array != nil {
			return result.Array, nil
		}
		caster, ok := opts.getCasterForPath(path)
		if !ok {
			return result.String, nil
		}
		casted, err := caster(result.String)
		if err != nil {
			return casted, newPathError(path, fmt.Errorf("failed to cast to expected type: %w", err))
		}
		return casted, nil

	case map[string]interface{}:
		out := map[string]interface{}{}
		for key, elem := range value {
			interpolatedElem, err := recursiveInterpolate(elem, path.Next(key), opts)
			if err != nil {
				return nil, err
			}
			out[key] = interpolatedElem
		}
		return out, nil

	case []interface{}:
		out := make([]interface{}, 0, len(value))
		for _, elem := range value {
			interpolatedElem, err := recursiveInterpolate(elem, path.Next(tree.PathMatchList), opts)
			if err != nil {
				return nil, err
			}
			if isStringSlice(interpolatedElem) {
				for _, nestedElem := range interpolatedElem.([]string) {
					out = append(out, nestedElem)
				}
			} else {
				out = append(out, interpolatedElem)
			}
		}
		return out, nil

	default:
		return value, nil
	}
}

func isStringSlice(value interface{}) bool {
	if value == nil {
		return false
	}
	t := reflect.TypeOf(value)
	if t.Kind() != reflect.Slice {
		return false
	}
	if t.Elem().Kind() != reflect.String {
		return false
	}
	return true
}

func newPathError(path tree.Path, err error) error {
	var ite *template.InvalidTemplateError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ite):
		return fmt.Errorf(
			"invalid interpolation format for %s.\nYou may need to escape any $ with another $.\n%s",
			path, ite.Template)
	default:
		return fmt.Errorf("error while interpolating %s:\n%w", path, err)
	}
}

func (o Options) getCasterForPath(path tree.Path) (Cast, bool) {
	for pattern, caster := range o.TypeCastMapping {
		if path.Matches(pattern) {
			return caster, true
		}
	}
	return nil, false
}
