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
	"os"
	"slices"
	"strings"

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
	Substitute func(string, template.Mapping) (string, error)
}

// LookupValue is a function which maps from variable names to values.
// Returns the value as a string and a bool indicating whether
// the value is present, to distinguish between an empty string
// and the absence of a value.
type LookupValue func(key string) (string, bool)

// Cast a value to a new type, or return an error if the value can't be cast
type Cast func(value string) (interface{}, error)

// Interpolate replaces variables in a string with the values from a mapping.
// Every failure is collected into a single joined error, reported in a stable
// order, rather than returning the first one. There is one error per failing
// config value: if a value holds several failing variable references, only
// the first one is reported. The walk continues past failures, so
// LookupValue, Substitute and Cast may run on values dropped from the result.
func Interpolate(config map[string]interface{}, opts Options) (map[string]interface{}, error) {
	if opts.LookupValue == nil {
		opts.LookupValue = os.LookupEnv
	}
	if opts.TypeCastMapping == nil {
		opts.TypeCastMapping = make(map[tree.Path]Cast)
	}
	if opts.Substitute == nil {
		opts.Substitute = template.Substitute
	}

	out := map[string]interface{}{}
	var errs []error
	for key, value := range config {
		interpolatedValue, err := recursiveInterpolate(value, tree.NewPath(key), opts)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out[key] = interpolatedValue
	}

	return out, joinErrors(errs)
}

func recursiveInterpolate(value interface{}, path tree.Path, opts Options) (interface{}, error) {
	switch value := value.(type) {
	case string:
		newValue, err := opts.Substitute(value, template.Mapping(opts.LookupValue))
		if err != nil {
			return value, newPathError(path, err)
		}
		caster, ok := opts.getCasterForPath(path)
		if !ok {
			return newValue, nil
		}
		casted, err := caster(newValue)
		if err != nil {
			return casted, newPathError(path, fmt.Errorf("failed to cast to expected type: %w", err))
		}
		return casted, nil

	case map[string]interface{}:
		out := map[string]interface{}{}
		var errs []error
		for key, elem := range value {
			interpolatedElem, err := recursiveInterpolate(elem, path.Next(key), opts)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			out[key] = interpolatedElem
		}
		return out, joinErrors(errs)

	case []interface{}:
		out := make([]interface{}, len(value))
		var errs []error
		for i, elem := range value {
			interpolatedElem, err := recursiveInterpolate(elem, path.Next(tree.PathMatchList), opts)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			out[i] = interpolatedElem
		}
		// Index order is already deterministic, no need to sort.
		return out, errors.Join(errs...)

	default:
		return value, nil
	}
}

// joinErrors joins errors collected while ranging over a map in a stable
// order, so that the random map iteration order does not leak into the
// reported error. Errors are sorted by their formatted message, so different
// error kinds are grouped rather than interleaved by config path. Only called
// on the error path: successful interpolation pays no sorting cost.
func joinErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	slices.SortStableFunc(errs, func(a, b error) int {
		return strings.Compare(a.Error(), b.Error())
	})
	return errors.Join(errs...)
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
		return fmt.Errorf("error while interpolating %s: %w", path, err)
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
