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

	"github.com/sirupsen/logrus"
)

var delimiter = "\\$"
var substitutionNamed = "[_a-z][_a-z0-9]*"
var substitutionBraced = "[_a-z][_a-z0-9]*(?::?[-+?](.*))?"
var substitutionMapping = "[_a-z][_a-z0-9]*\\[(.*)\\](?::?[-+?}](.*))?"

var groupEscaped = "escaped"
var groupNamed = "named"
var groupBraced = "braced"
var groupMapping = "mapping"
var groupInvalid = "invalid"

var patternString = fmt.Sprintf(
	"%s(?i:(?P<%s>%s)|(?P<%s>%s)|{(?:(?P<%s>%s)}|(?P<%s>%s)}|(?P<%s>)))",
	delimiter,
	groupEscaped, delimiter,
	groupNamed, substitutionNamed,
	groupBraced, substitutionBraced,
	groupMapping, substitutionMapping,
	groupInvalid,
)

var DefaultPattern = regexp.MustCompile(patternString)

var NamedMappingKeyPattern = regexp.MustCompile("^[_A-Za-z0-9.-]*$")

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

// MissingNamedMappingError is returned when a specific named mapping is missing
// Guaranteed to not return if `Config.namedMappings` is nil (named mappings not enabled)
type MissingNamedMappingError struct {
	Name string
}

func (e MissingNamedMappingError) Error() string {
	return fmt.Sprintf("named mapping not found: %q", e.Name)
}

// Mapping is a user-supplied function which maps from variable names to values.
// Returns the value as a string and a bool indicating whether
// the value is present, to distinguish between an empty string
// and the absence of a value.
type Mapping func(string) (string, bool)

// NamedMappings is a collection of mappings indexed by a name key.
// It allows temporarily switching to other mappings other than default during interpolation
type NamedMappings map[string]Mapping

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
	namedMappings   NamedMappings
	logging         bool
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

func WithNamedMappings(namedMappings NamedMappings) Option {
	return func(cfg *Config) {
		cfg.namedMappings = namedMappings
	}
}

func WithoutLogging(cfg *Config) {
	cfg.logging = false
}

// SubstituteWithOptions substitute variables in the string with their values.
// It accepts additional options such as a custom function or pattern.
func SubstituteWithOptions(template string, mapping Mapping, options ...Option) (string, error) {
	cfg := &Config{
		pattern:         DefaultPattern,
		replacementFunc: DefaultReplacementFunc,
		logging:         true,
	}
	for _, o := range options {
		o(cfg)
	}

	return substituteWithConfig(template, mapping, cfg)
}

func substituteWithConfig(template string, mapping Mapping, cfg *Config) (result string, returnErr error) {
	if cfg == nil {
		cfg = &Config{
			pattern:         DefaultPattern,
			replacementFunc: DefaultReplacementFunc,
			logging:         true,
		}
	}

	defer func() {
		// Convert panic message to error, so mappings can use panic to report error
		if r := recover(); r != nil {
			switch r := r.(type) {
			case string:
				returnErr = errors.New(r)
			case error:
				returnErr = r
			default:
				returnErr = errors.New(fmt.Sprint(r))
			}
		}
	}()

	result = cfg.pattern.ReplaceAllStringFunc(template, func(substring string) string {
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

func DefaultReplacementAppliedFunc(substring string, mapping Mapping, cfg *Config) (value string, applied bool, err error) {
	template := substring
	pattern := cfg.pattern

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

	substitution := ""
	subsFunc := cfg.substituteFunc
	switch {
	case groups[groupNamed] != "":
		substitution = groups[groupNamed]
	case groups[groupBraced] != "":
		substitution = groups[groupBraced]
		if subsFunc == nil {
			_, subsFunc = getSubstitutionFunctionForTemplate(template, cfg)
		}
	case groups[groupMapping] != "":
		substitution = groups[groupMapping]
		if subsFunc == nil {
			subsFunc = getSubstitutionFunctionForNamedMapping(cfg)
		}
	default:
		return "", false, &InvalidTemplateError{}
	}

	if subsFunc != nil {
		value, applied, err = subsFunc(substitution, mapping)
		if err != nil {
			return "", false, err
		}
		if !applied {
			value = substring // Keep the original substring ${...} if not applied
		}
	} else {
		value, applied = mapping(substitution)
		if !applied && cfg.logging {
			logrus.Warnf("The %q variable is not set. Defaulting to a blank string.", substitution)
		}
	}

	if rest != "" {
		interpolatedNested, err := substituteWithConfig(rest, mapping, cfg)
		applied = applied || rest != interpolatedNested
		if err != nil {
			return "", false, err
		}
		value += interpolatedNested
	}

	return value, applied, nil
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

func getSubstitutionFunctionForTemplate(template string, cfg *Config) (string, SubstituteFunc) {
	interpolationMapping := []struct {
		SubstituteType string
		SubstituteFunc func(string, Mapping, *Config) (string, bool, error)
	}{
		{":?", requiredErrorWhenEmptyOrUnset},
		{"?", requiredErrorWhenUnset},
		{":-", defaultWhenEmptyOrUnset},
		{"-", defaultWhenUnset},
		{":+", defaultWhenNotEmpty},
		{"+", defaultWhenSet},
	}

	mappingIndices := make(map[string]int)
	hasInterpolationMapping := false
	for _, m := range interpolationMapping {
		mappingIndices[m.SubstituteType] = strings.Index(template, m.SubstituteType)
		if mappingIndices[m.SubstituteType] >= 0 {
			hasInterpolationMapping = true
		}
	}
	if !hasInterpolationMapping {
		return "", nil
	}

	sort.Slice(interpolationMapping, func(i, j int) bool {
		idxI := mappingIndices[interpolationMapping[i].SubstituteType]
		idxJ := mappingIndices[interpolationMapping[j].SubstituteType]
		if idxI < 0 {
			return false
		}
		if idxJ < 0 {
			return true
		}
		return idxI < idxJ
	})

	return interpolationMapping[0].SubstituteType, func(s string, m Mapping) (string, bool, error) {
		return interpolationMapping[0].SubstituteFunc(s, m, cfg)
	}
}

func getSubstitutionFunctionForNamedMapping(cfg *Config) SubstituteFunc {
	return func(substitution string, mapping Mapping) (string, bool, error) {
		namedMapping, key, rest, err := getNamedMapping(substitution, cfg)
		if err != nil || namedMapping == nil {
			return "", false, err
		}

		resolvedKey, err := getResolvedNamedMappingKey(key, mapping, cfg)
		if err != nil {
			return "", false, err
		}

		// If subsitution function found, delegate substitution string (with key resolved) to it
		if rest != "" {
			subsType, subsFunc := getSubstitutionFunctionForTemplate(rest, cfg)
			if subsType == "" {
				return "", false, &InvalidTemplateError{Template: substitution}
			}
			substitution := strings.Replace(substitution, key, resolvedKey, 1)
			value, applied, err := subsFunc(substitution, mapping)
			if applied || err != nil {
				return value, applied, err
			}
		}

		value, _ := namedMapping(resolvedKey)
		return value, true, nil
	}
}

func getNamedMapping(substitution string, cfg *Config) (Mapping, string, string, error) {
	if cfg.namedMappings == nil { // Named mappings not enabled
		return nil, "", "", nil
	}

	openBracketIndex := -1
	closeBracketIndex := -1
	openBrackets := 0
	for i := 0; i < len(substitution); i++ {
		if substitution[i] == '[' {
			if openBrackets == 0 {
				openBracketIndex = i
			}
			openBrackets += 1
		} else if substitution[i] == ']' {
			openBrackets -= 1
			if openBrackets == 0 {
				closeBracketIndex = i
			}
			if openBrackets <= 0 {
				break
			}
		}
	}
	if openBracketIndex < 0 || closeBracketIndex < 0 {
		return nil, "", "", nil
	}
	name := substitution[0:openBracketIndex]
	key := substitution[openBracketIndex+1 : closeBracketIndex]
	rest := substitution[closeBracketIndex+1:]

	namedMapping, ok := cfg.namedMappings[name]
	if !ok { // When namd mappings config provided, it must be able to resolve all mapping names in the template
		return nil, "", "", &MissingNamedMappingError{Name: name}
	}

	return namedMapping, key, rest, nil
}

func getResolvedNamedMappingKey(key string, mapping Mapping, cfg *Config) (string, error) {
	resolvedKey, err := substituteWithConfig(key, mapping, cfg)
	if err != nil {
		return "", err
	}

	if !NamedMappingKeyPattern.MatchString(resolvedKey) {
		if resolvedKey != key {
			return "", fmt.Errorf("invalid key in named mapping: %q (resolved to %q)", key, resolvedKey)
		} else {
			return "", fmt.Errorf("invalid key in named mapping: %q", key)
		}
	}

	return resolvedKey, nil
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

// Soft default (fall back if unset or empty)
func defaultWhenEmptyOrUnset(substitution string, mapping Mapping, cfg *Config) (string, bool, error) {
	return withDefaultWhenAbsence(substitution, mapping, cfg, true)
}

// Hard default (fall back if-and-only-if empty)
func defaultWhenUnset(substitution string, mapping Mapping, cfg *Config) (string, bool, error) {
	return withDefaultWhenAbsence(substitution, mapping, cfg, false)
}

func defaultWhenNotEmpty(substitution string, mapping Mapping, cfg *Config) (string, bool, error) {
	return withDefaultWhenPresence(substitution, mapping, cfg, true)
}

func defaultWhenSet(substitution string, mapping Mapping, cfg *Config) (string, bool, error) {
	return withDefaultWhenPresence(substitution, mapping, cfg, false)
}

func requiredErrorWhenEmptyOrUnset(substitution string, mapping Mapping, cfg *Config) (string, bool, error) {
	return withRequired(substitution, mapping, cfg, ":?", func(v string) bool { return v != "" })
}

func requiredErrorWhenUnset(substitution string, mapping Mapping, cfg *Config) (string, bool, error) {
	return withRequired(substitution, mapping, cfg, "?", func(_ string) bool { return true })
}

func withDefaultWhenPresence(substitution string, mapping Mapping, cfg *Config, notEmpty bool) (value string, ok bool, err error) {
	sep := "+"
	if notEmpty {
		sep = ":+"
	}
	if !strings.Contains(substitution, sep) {
		return "", false, nil
	}
	name, defaultValue := partition(substitution, sep)
	defaultValue, err = substituteWithConfig(defaultValue, mapping, cfg)
	if err != nil {
		return "", false, err
	}
	namedMapping, key, rest, err := getNamedMapping(name, cfg)
	if err != nil {
		return "", false, err
	}
	if rest != "" {
		return "", false, &InvalidTemplateError{Template: substitution}
	}
	if namedMapping != nil {
		value, ok = namedMapping(key)
	} else {
		value, ok = mapping(name)
	}
	if ok && (!notEmpty || (notEmpty && value != "")) {
		return defaultValue, true, nil
	}
	return value, true, nil
}

func withDefaultWhenAbsence(substitution string, mapping Mapping, cfg *Config, emptyOrUnset bool) (value string, ok bool, err error) {
	sep := "-"
	if emptyOrUnset {
		sep = ":-"
	}
	if !strings.Contains(substitution, sep) {
		return "", false, nil
	}
	name, defaultValue := partition(substitution, sep)
	defaultValue, err = substituteWithConfig(defaultValue, mapping, cfg)
	if err != nil {
		return "", false, err
	}
	namedMapping, key, rest, err := getNamedMapping(name, cfg)
	if err != nil {
		return "", false, err
	}
	if rest != "" {
		return "", false, &InvalidTemplateError{Template: substitution}
	}
	if namedMapping != nil {
		value, ok = namedMapping(key)
	} else {
		value, ok = mapping(name)
	}
	if !ok || (emptyOrUnset && value == "") {
		return defaultValue, true, nil
	}
	return value, true, nil
}

func withRequired(substitution string, mapping Mapping, cfg *Config, sep string, valid func(string) bool) (value string, ok bool, err error) {
	if !strings.Contains(substitution, sep) {
		return "", false, nil
	}
	name, errorMessage := partition(substitution, sep)
	errorMessage, err = substituteWithConfig(errorMessage, mapping, cfg)
	if err != nil {
		return "", false, err
	}
	namedMapping, key, rest, err := getNamedMapping(name, cfg)
	if err != nil {
		return "", false, err
	}
	if rest != "" {
		return "", false, &InvalidTemplateError{Template: substitution}
	}
	if namedMapping != nil {
		value, ok = namedMapping(key)
	} else {
		value, ok = mapping(name)
	}
	if !ok || !valid(value) {
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

// Merge stacks new mapping on top of current mapping, i.e. the merged mapping will:
// 1. Lookup in current mapping first
// 2. If not present in current mapping, then lookup in provided mapping
func (m Mapping) Merge(other Mapping) Mapping {
	return func(key string) (string, bool) {
		if value, ok := m(key); ok {
			return value, ok
		}
		return other(key)
	}
}

func (m NamedMappings) Merge(other NamedMappings) NamedMappings {
	if m == nil {
		return other
	}
	if other == nil {
		return m
	}
	merged := make(NamedMappings)
	for name, mapping := range m {
		if otherMapping, ok := other[name]; ok {
			merged[name] = mapping.Merge(otherMapping)
		} else {
			merged[name] = mapping
		}
	}
	for name, mapping := range other {
		if _, ok := merged[name]; !ok {
			merged[name] = mapping
		}
	}
	return merged
}
