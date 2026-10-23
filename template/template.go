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

package template

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/compose-spec/compose-go/v2/internal/warn"
	"github.com/compose-spec/compose-go/v2/logging"
)

const (
	delimiter          = "\\$"
	substitutionNamed  = "[_a-z][_a-z0-9]*"
	substitutionBraced = "[_a-z][_a-z0-9]*(?::?[-+?](.*))?"
	groupEscaped       = "escaped"
	groupNamed         = "named"
	groupBraced        = "braced"
	groupInvalid       = "invalid"
)

var (
	patternString = fmt.Sprintf(
		"%s(?i:(?P<%s>%s)|(?P<%s>%s)|{(?:(?P<%s>%s)}|(?P<%s>)))",
		delimiter,
		groupEscaped, delimiter,
		groupNamed, substitutionNamed,
		groupBraced, substitutionBraced,
		groupInvalid,
	)

	DefaultPattern = regexp.MustCompile(patternString)
)

// InvalidTemplateError is returned when a variable template is not in a valid
// format
type InvalidTemplateError struct {
	Template string
}

func (e InvalidTemplateError) Error() string {
	return fmt.Sprintf("Invalid template: %#v", e.Template)
}

// MissingRequiredError is returned when a variable template is missing
type MissingRequiredError struct {
	Variable string
	Reason   string
}

func (e MissingRequiredError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("required variable %s is missing a value: %s", e.Variable, e.Reason)
	}
	return fmt.Sprintf("required variable %s is missing a value", e.Variable)
}

// Mapping is a user-supplied function which maps from variable names to values.
// Returns the value as a string and a bool indicating whether
// the value is present, to distinguish between an empty string
// and the absence of a value.
type Mapping func(string) (string, bool)

// SubstituteFunc is a user-supplied function that apply substitution.
// Returns the value as a string, a bool indicating if the function could apply
// the substitution and an error.
type SubstituteFunc func(string, Mapping) (string, bool, error)

// ReplacementFunc is a user-supplied function that is apply to the matching
// substring. Returns the value as a string and an error.
type ReplacementFunc func(string, Mapping, *Config) (string, error)

type Config struct {
	pattern         *regexp.Regexp
	substituteFunc  SubstituteFunc
	replacementFunc ReplacementFunc
	logging         bool
	logger          logging.Logger
}

type Option func(*Config)

func WithPattern(pattern *regexp.Regexp) Option {
	return func(cfg *Config) {
		cfg.pattern = pattern
	}
}

func WithSubstitutionFunction(subsFunc SubstituteFunc) Option {
	return func(cfg *Config) {
		cfg.substituteFunc = subsFunc
	}
}

func WithReplacementFunction(replacementFunc ReplacementFunc) Option {
	return func(cfg *Config) {
		cfg.replacementFunc = replacementFunc
	}
}

func WithoutLogging(cfg *Config) {
	cfg.logging = false
}

// WithLogger sends the warnings emitted during substitution (a variable that
// is not set) to l instead of the standard logrus logger. It has no effect
// when combined with WithoutLogging.
func WithLogger(l logging.Logger) Option {
	return func(cfg *Config) {
		cfg.logger = l
	}
}

// SubstituteWithOptions substitute variables in the string with their values.
// It accepts additional options such as a custom function or pattern.
func SubstituteWithOptions(template string, mapping Mapping, options ...Option) (string, error) {
	var returnErr error

	cfg := &Config{
		pattern:         DefaultPattern,
		replacementFunc: DefaultReplacementFunc,
		logging:         true,
	}
	for _, o := range options {
		o(cfg)
	}

	result := cfg.pattern.ReplaceAllStringFunc(template, func(substring string) string {
		replacement, err := cfg.replacementFunc(substring, mapping, cfg)
		if err != nil {
			// Add the template for template errors
			var tmplErr *InvalidTemplateError
			if errors.As(err, &tmplErr) {
				if tmplErr.Template == "" {
					tmplErr.Template = template
				}
			}
			// Save the first error to be returned
			if returnErr == nil {
				returnErr = err
			}

		}
		return replacement
	})

	return result, returnErr
}

func DefaultReplacementFunc(substring string, mapping Mapping, cfg *Config) (string, error) {
	value, _, err := DefaultReplacementAppliedFunc(substring, mapping, cfg)
	return value, err
}

func DefaultReplacementAppliedFunc(substring string, mapping Mapping, cfg *Config) (string, bool, error) {
	pattern := cfg.pattern
	subsFunc := cfg.substituteFunc
	if subsFunc == nil {
		// nested substitutions (default values, error messages) report to the
		// same logger as this one
		_, subsFunc = substitutionFunctionForTemplate(substring, func(t string, m Mapping) (string, error) {
			return SubstituteWithOptions(t, m, nestedLogging(cfg))
		})
	}

	closingBraceIndex := getFirstBraceClosingIndex(substring)
	rest := ""
	if closingBraceIndex > -1 {
		rest = substring[closingBraceIndex+1:]
		substring = substring[0 : closingBraceIndex+1]
	}

	matches := pattern.FindStringSubmatch(substring)
	groups := matchGroups(matches, pattern)
	if escaped := groups[groupEscaped]; escaped != "" {
		return escaped, true, nil
	}

	braced := false
	substitution := groups[groupNamed]
	if substitution == "" {
		substitution = groups[groupBraced]
		braced = true
	}

	if substitution == "" {
		return "", false, &InvalidTemplateError{}
	}

	if braced {
		value, applied, err := subsFunc(substitution, mapping)
		if err != nil {
			return "", false, err
		}
		if applied {
			interpolatedNested, err := SubstituteWithOptions(rest, mapping, WithPattern(pattern), nestedLogging(cfg))
			if err != nil {
				return "", false, err
			}
			return value + interpolatedNested, true, nil
		}
	}

	value, ok := mapping(substitution)
	if !ok && cfg.logging {
		warn.Warnf(cfg.logger, "The %q variable is not set. Defaulting to a blank string.", substitution)
	}

	return value, ok, nil
}

// nestedLogging makes a nested substitution report like the one it belongs to:
// to the same logger, and not at all when logging is disabled.
func nestedLogging(cfg *Config) Option {
	return func(nested *Config) {
		nested.logger = cfg.logger
		nested.logging = cfg.logging
	}
}

// SubstituteWith substitute variables in the string with their values.
// It accepts additional substitute function.
func SubstituteWith(template string, mapping Mapping, pattern *regexp.Regexp, subsFuncs ...SubstituteFunc) (string, error) {
	options := []Option{
		WithPattern(pattern),
	}
	if len(subsFuncs) > 0 {
		options = append(options, WithSubstitutionFunction(subsFuncs[0]))
	}

	return SubstituteWithOptions(template, mapping, options...)
}

func getSubstitutionFunctionForTemplate(template string) (string, SubstituteFunc) {
	return substitutionFunctionForTemplate(template, Substitute)
}

// substitutionFunctionForTemplate selects the substitution function matching
// the operator found first in template. nested is used to substitute the
// default value or error message the operator carries.
func substitutionFunctionForTemplate(template string, nested nestedSubstitute) (string, SubstituteFunc) {
	interpolationMapping := []struct {
		string
		SubstituteFunc
	}{
		{":?", requiredErrorWhenEmptyOrUnset(nested)},
		{"?", requiredErrorWhenUnset(nested)},
		{":-", defaultWhenEmptyOrUnset(nested)},
		{"-", defaultWhenUnset(nested)},
		{":+", defaultWhenNotEmpty(nested)},
		{"+", defaultWhenSet(nested)},
	}
	sort.Slice(interpolationMapping, func(i, j int) bool {
		idxI := strings.Index(template, interpolationMapping[i].string)
		idxJ := strings.Index(template, interpolationMapping[j].string)
		if idxI < 0 {
			return false
		}
		if idxJ < 0 {
			return true
		}
		return idxI < idxJ
	})

	return interpolationMapping[0].string, interpolationMapping[0].SubstituteFunc
}

func getFirstBraceClosingIndex(s string) int {
	openVariableBraces := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '}' {
			openVariableBraces--
			if openVariableBraces == 0 {
				return i
			}
		}
		if s[i] == '{' {
			openVariableBraces++
			i++
		}
	}
	return -1
}

// Substitute variables in the string with their values
func Substitute(template string, mapping Mapping) (string, error) {
	return SubstituteWith(template, mapping, DefaultPattern)
}

// nestedSubstitute substitutes the variables found in a default value or an
// error message.
type nestedSubstitute func(string, Mapping) (string, error)

// Soft default (fall back if unset or empty)
func defaultWhenEmptyOrUnset(nested nestedSubstitute) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		return withDefaultWhenAbsence(substitution, mapping, true, nested)
	}
}

// Hard default (fall back if-and-only-if empty)
func defaultWhenUnset(nested nestedSubstitute) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		return withDefaultWhenAbsence(substitution, mapping, false, nested)
	}
}

func defaultWhenNotEmpty(nested nestedSubstitute) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		return withDefaultWhenPresence(substitution, mapping, true, nested)
	}
}

func defaultWhenSet(nested nestedSubstitute) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		return withDefaultWhenPresence(substitution, mapping, false, nested)
	}
}

func requiredErrorWhenEmptyOrUnset(nested nestedSubstitute) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		return withRequired(substitution, mapping, ":?", func(v string) bool { return v != "" }, nested)
	}
}

func requiredErrorWhenUnset(nested nestedSubstitute) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		return withRequired(substitution, mapping, "?", func(_ string) bool { return true }, nested)
	}
}

func withDefaultWhenPresence(substitution string, mapping Mapping, notEmpty bool, nested nestedSubstitute) (string, bool, error) {
	sep := "+"
	if notEmpty {
		sep = ":+"
	}
	if !strings.Contains(substitution, sep) {
		return "", false, nil
	}
	name, defaultValue := partition(substitution, sep)
	value, ok := mapping(name)
	if ok && (!notEmpty || (notEmpty && value != "")) {
		defaultValue, err := nested(defaultValue, mapping)
		if err != nil {
			return "", false, err
		}
		return defaultValue, true, nil
	}
	return value, true, nil
}

func withDefaultWhenAbsence(substitution string, mapping Mapping, emptyOrUnset bool, nested nestedSubstitute) (string, bool, error) {
	sep := "-"
	if emptyOrUnset {
		sep = ":-"
	}
	if !strings.Contains(substitution, sep) {
		return "", false, nil
	}
	name, defaultValue := partition(substitution, sep)
	value, ok := mapping(name)
	if !ok || (emptyOrUnset && value == "") {
		defaultValue, err := nested(defaultValue, mapping)
		if err != nil {
			return "", false, err
		}
		return defaultValue, true, nil
	}
	return value, true, nil
}

func withRequired(substitution string, mapping Mapping, sep string, valid func(string) bool, nested nestedSubstitute) (string, bool, error) {
	if !strings.Contains(substitution, sep) {
		return "", false, nil
	}
	name, errorMessage := partition(substitution, sep)
	value, ok := mapping(name)
	if !ok || !valid(value) {
		errorMessage, err := nested(errorMessage, mapping)
		if err != nil {
			return "", false, err
		}
		return "", true, &MissingRequiredError{
			Reason:   errorMessage,
			Variable: name,
		}
	}
	return value, true, nil
}

func matchGroups(matches []string, pattern *regexp.Regexp) map[string]string {
	groups := make(map[string]string)
	for i, name := range pattern.SubexpNames()[1:] {
		groups[name] = matches[i+1]
	}
	return groups
}

// Split the string at the first occurrence of sep, and return the part before the separator,
// and the part after the separator.
//
// If the separator is not found, return the string itself, followed by an empty string.
func partition(s, sep string) (string, string) {
	if strings.Contains(s, sep) {
		parts := strings.SplitN(s, sep, 2)
		return parts[0], parts[1]
	}
	return s, ""
}
