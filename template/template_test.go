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
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

var defaults = map[string]string{
	"FOO":  "first",
	"BAR":  "",
	"JSON": `{"json":2}`,
}

func defaultMapping(name string) (string, bool) {
	val, ok := defaults[name]
	return val, ok
}

func TestEscaped(t *testing.T) {
	result, err := Substitute("$${foo}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("${foo}", result))
}

func TestSubstituteNoMatch(t *testing.T) {
	result, err := Substitute("foo", defaultMapping)
	assert.NilError(t, err)
	assert.Equal(t, "foo", result)
}

func TestUnescaped(t *testing.T) {
	templates := []string{
		"a $ string",
		"^REGEX$",
		"$}",
		"$",
	}

	for _, expected := range templates {
		actual, err := Substitute(expected, defaultMapping)
		assert.NilError(t, err)
		assert.Equal(t, expected, actual)
	}
}

func TestInvalid(t *testing.T) {
	invalidTemplates := []string{
		"${",
		"${}",
		"${ }",
		"${ foo}",
		"${foo }",
		"${foo!}",
	}

	for _, template := range invalidTemplates {
		_, err := Substitute(template, defaultMapping)
		assert.ErrorContains(t, err, "Invalid template")
	}
}

// see https://github.com/docker/compose/issues/8601
func TestNonBraced(t *testing.T) {
	substituted, err := Substitute("$FOO-bar", defaultMapping)
	assert.NilError(t, err)
	assert.Equal(t, substituted, "first-bar")
}

func TestNoValueNoDefault(t *testing.T) {
	for _, template := range []string{"This ${missing} var", "This ${BAR} var"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal("This  var", result))
	}
}

func TestValueNoDefault(t *testing.T) {
	for _, template := range []string{"This $FOO var", "This ${FOO} var"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal("This first var", result))
	}
}

func TestNoValueWithDefault(t *testing.T) {
	for _, template := range []string{"ok ${missing:-def}", "ok ${missing-def}"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal("ok def", result))
	}
}

func TestEmptyValueWithSoftDefault(t *testing.T) {
	result, err := Substitute("ok ${BAR:-def}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok def", result))
}

func TestValueWithSoftDefault(t *testing.T) {
	result, err := Substitute("ok ${FOO:-def}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))
}

func TestEmptyValueWithHardDefault(t *testing.T) {
	result, err := Substitute("ok ${BAR-def}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))
}

func TestPresentValueWithUnset(t *testing.T) {
	result, err := Substitute("ok ${UNSET_VAR:+presence_value}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))
}

func TestPresentValueWithUnset2(t *testing.T) {
	result, err := Substitute("ok ${UNSET_VAR+presence_value}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))
}

func TestPresentValueWithNonEmpty(t *testing.T) {
	result, err := Substitute("ok ${FOO:+presence_value}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok presence_value", result))
}

func TestPresentValueAndNonEmptyWithNonEmpty(t *testing.T) {
	result, err := Substitute("ok ${FOO+presence_value}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok presence_value", result))
}

func TestPresentValueWithSet(t *testing.T) {
	result, err := Substitute("ok ${BAR+presence_value}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok presence_value", result))
}

func TestPresentValueAndNotEmptyWithSet(t *testing.T) {
	result, err := Substitute("ok ${BAR:+presence_value}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))
}

func TestNonAlphanumericDefault(t *testing.T) {
	result, err := Substitute("ok ${BAR:-/non:-alphanumeric}", defaultMapping)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok /non:-alphanumeric", result))
}

func TestInterpolationExternalInterference(t *testing.T) {
	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			template: "-ok ${BAR:-defaultValue}",
			expected: "-ok defaultValue",
		},
		{
			template: "+ok ${UNSET:-${BAR-defaultValue}}",
			expected: "+ok ",
		},
		{
			template: "-ok ${FOO:-defaultValue}",
			expected: "-ok first",
		},
		{
			template: ":-ok ${UNSET-defaultValue}",
			expected: ":-ok defaultValue",
		},
		{
			template: ":-ok ${BAR-defaultValue}",
			expected: ":-ok ",
		},
		{
			template: ":?ok ${BAR-defaultValue}",
			expected: ":?ok ",
		},
		{
			template: ":?ok ${BAR:-defaultValue}",
			expected: ":?ok defaultValue",
		},
		{
			template: ":+ok ${BAR:-defaultValue}",
			expected: ":+ok defaultValue",
		},
		{
			template: "+ok ${BAR-defaultValue}",
			expected: "+ok ",
		},
		{
			template: "?ok ${BAR:-defaultValue}",
			expected: "?ok defaultValue",
		},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("Interpolation Should not be impacted by outer text: %d", i), func(t *testing.T) {
			result, err := Substitute(tc.template, defaultMapping)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(tc.expected, result))
		})
	}
}

func TestDefaultsWithNestedExpansion(t *testing.T) {
	testCases := []struct {
		template string
		expected string
	}{
		{
			template: "ok ${UNSET_VAR-$FOO}",
			expected: "ok first",
		},
		{
			template: "ok ${UNSET_VAR-${FOO}}",
			expected: "ok first",
		},
		{
			template: "ok ${UNSET_VAR-${FOO} ${FOO}}",
			expected: "ok first first",
		},
		{
			template: "ok ${BAR:-$FOO}",
			expected: "ok first",
		},
		{
			template: "ok ${BAR:-${FOO}}",
			expected: "ok first",
		},
		{
			template: "ok ${BAR:-${FOO} ${FOO}}",
			expected: "ok first first",
		},
		{
			template: "ok ${BAR+$FOO}",
			expected: "ok first",
		},
		{
			template: "ok ${BAR+$FOO ${FOO:+second}}",
			expected: "ok first second",
		},
	}

	for _, tc := range testCases {
		result, err := Substitute(tc.template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(tc.expected, result))
	}
}

func TestMandatoryVariableErrors(t *testing.T) {
	testCases := []struct {
		template      string
		expectedError string
	}{
		{
			template:      "not ok ${UNSET_VAR:?Mandatory Variable Unset}",
			expectedError: "required variable UNSET_VAR is missing a value: Mandatory Variable Unset",
		},
		{
			template:      "not ok ${BAR:?Mandatory Variable Empty}",
			expectedError: "required variable BAR is missing a value: Mandatory Variable Empty",
		},
		{
			template:      "not ok ${UNSET_VAR:?}",
			expectedError: "required variable UNSET_VAR is missing a value",
		},
		{
			template:      "not ok ${UNSET_VAR?Mandatory Variable Unset}",
			expectedError: "required variable UNSET_VAR is missing a value: Mandatory Variable Unset",
		},
		{
			template:      "not ok ${UNSET_VAR?}",
			expectedError: "required variable UNSET_VAR is missing a value",
		},
	}

	for _, tc := range testCases {
		_, err := Substitute(tc.template, defaultMapping)
		assert.ErrorContains(t, err, tc.expectedError)
		assert.ErrorType(t, err, reflect.TypeOf(&MissingRequiredError{}))
	}
}

func TestMandatoryVariableErrorsWithNestedExpansion(t *testing.T) {
	testCases := []struct {
		template      string
		expectedError string
	}{
		{
			template:      "not ok ${UNSET_VAR:?Mandatory Variable ${FOO}}",
			expectedError: "required variable UNSET_VAR is missing a value: Mandatory Variable first",
		},
		{
			template:      "not ok ${UNSET_VAR?Mandatory Variable ${FOO}}",
			expectedError: "required variable UNSET_VAR is missing a value: Mandatory Variable first",
		},
	}

	for _, tc := range testCases {
		_, err := Substitute(tc.template, defaultMapping)
		assert.ErrorContains(t, err, tc.expectedError)
		assert.ErrorType(t, err, reflect.TypeOf(&MissingRequiredError{}))
	}
}

func TestDefaultsForMandatoryVariables(t *testing.T) {
	testCases := []struct {
		template string
		expected string
	}{
		{
			template: "ok ${FOO:?err}",
			expected: "ok first",
		},
		{
			template: "ok ${FOO?err}",
			expected: "ok first",
		},
		{
			template: "ok ${BAR?err}",
			expected: "ok ",
		},
	}

	for _, tc := range testCases {
		result, err := Substitute(tc.template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(tc.expected, result))
	}
}

func TestSubstituteWithCustomFunc(t *testing.T) {
	errIsMissing := func(substitution string, mapping Mapping) (string, bool, error) {
		value, found := mapping(substitution)
		if !found {
			return "", true, &InvalidTemplateError{
				Template: fmt.Sprintf("required variable %s is missing a value", substitution),
			}
		}
		return value, true, nil
	}

	result, err := SubstituteWith("ok ${FOO}", defaultMapping, DefaultPattern, errIsMissing)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))

	result, err = SubstituteWith("ok ${BAR}", defaultMapping, DefaultPattern, errIsMissing)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))

	_, err = SubstituteWith("ok ${NOTHERE}", defaultMapping, DefaultPattern, errIsMissing)
	assert.Check(t, is.ErrorContains(err, "required variable"))
}

func TestSubstituteWithReplacementFunc(t *testing.T) {
	options := []Option{
		WithReplacementFunction(func(s string, m Mapping, c *Config) (string, error) {
			if s == "${NOTHERE}" {
				return "", fmt.Errorf("bad choice: %q", s)
			}
			r, err := DefaultReplacementFunc(s, m, c)
			if err == nil && r != "" {
				return r, nil
			}
			return "foobar", nil
		}),
	}
	result, err := SubstituteWithOptions("ok ${FOO}", defaultMapping, options...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))

	result, err = SubstituteWithOptions("ok ${BAR}", defaultMapping, options...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok foobar", result))

	result, err = SubstituteWithOptions("ok ${UNSET}", defaultMapping, options...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok foobar", result))

	_, err = SubstituteWithOptions("ok ${NOTHERE}", defaultMapping, options...)
	assert.Check(t, is.ErrorContains(err, "bad choice"))

	result, err = SubstituteWith("ok ${SUBDOMAIN:-redis}.${FOO:?}", defaultMapping, DefaultPattern)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok redis.first", result))
}

func TestSubstituteWithReplacementAppliedFunc(t *testing.T) {
	options := []Option{
		WithReplacementFunction(func(s string, m Mapping, c *Config) (string, error) {
			if s == "${NOTHERE}" {
				return "", fmt.Errorf("bad choice: %q", s)
			}
			r, applied, err := DefaultReplacementAppliedFunc(s, m, c)
			if err == nil && applied {
				return r, nil
			}
			return "foobar", nil
		}),
	}
	result, err := SubstituteWithOptions("ok ${FOO}", defaultMapping, options...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))

	result, err = SubstituteWithOptions("ok ${BAR}", defaultMapping, options...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))

	result, err = SubstituteWithOptions("ok ${UNSET}", defaultMapping, options...)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok foobar", result))

	_, err = SubstituteWithOptions("ok ${NOTHERE}", defaultMapping, options...)
	assert.Check(t, is.ErrorContains(err, "bad choice"))
}

// TestPrecedence tests is the precedence on '-' and '?' is of the first match
func TestPrecedence(t *testing.T) {
	testCases := []struct {
		template string
		expected string
		err      error
	}{
		{
			template: "${UNSET_VAR?bar-baz}", // Unexistent variable
			expected: "",
			err: &MissingRequiredError{
				Variable: "UNSET_VAR",
				Reason:   "bar-baz",
			},
		},
		{
			template: "${UNSET_VAR-myerror?msg}", // Unexistent variable
			expected: "myerror?msg",
			err:      nil,
		},

		{
			template: "${FOO?bar-baz}", // Existent variable
			expected: "first",
		},
		{
			template: "${BAR:-default_value_for_empty_var}", // Existent empty variable
			expected: "default_value_for_empty_var",
		},
		{
			template: "${UNSET_VAR-default_value_for_unset_var}", // Unset variable
			expected: "default_value_for_unset_var",
		},
	}

	for _, tc := range testCases {
		result, err := Substitute(tc.template, defaultMapping)
		assert.Check(t, is.DeepEqual(tc.err, err))
		assert.Check(t, is.Equal(tc.expected, result))
	}
}

func TestSubstitutionFunctionChoice(t *testing.T) {
	testcases := []struct {
		name   string
		input  string
		symbol string
	}{
		{"Error when EMPTY or UNSET", "VARNAME:?val?ue", ":?"},
		{"Error when UNSET 1", "VARNAME?val:?ue", "?"},
		{"Error when UNSET 2", "VARNAME?va-lu+e:?e", "?"},
		{"Error when UNSET 3", "VARNAME?va+lu-e:?e", "?"},

		{"Default when EMPTY or UNSET", "VARNAME:-value", ":-"},
		{"Default when UNSET 1", "VARNAME-va:-lu:?e", "-"},
		{"Default when UNSET 2", "VARNAME-va+lu?e", "-"},
		{"Default when UNSET 3", "VARNAME-va?lu+e", "-"},

		{"Default when NOT EMPTY", "VARNAME:+va:?lu:-e", ":+"},
		{"Default when SET 1", "VARNAME+va:+lue", "+"},
		{"Default when SET 2", "VARNAME+va?lu-e", "+"},
		{"Default when SET 3", "VARNAME+va-lu?e", "+"},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			symbol, _ := getSubstitutionFunctionForTemplate(tc.input, nil)
			assert.Equal(t, symbol, tc.symbol,
				fmt.Sprintf("Wrong on output for: %s got symbol -> %#v", tc.input, symbol),
			)
		})
	}
}

func TestNoValueWithCurlyBracesDefault(t *testing.T) {
	for _, template := range []string{`ok ${missing:-{"json":1}}`, `ok ${missing-{"json":1}}`} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(`ok {"json":1}`, result))
	}
}

func TestValueWithCurlyBracesDefault(t *testing.T) {
	for _, template := range []string{`ok ${JSON:-{"json":1}}`, `ok ${JSON-{"json":1}}`} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(`ok {"json":2}`, result))
	}
}

func envMapping(name string) (string, bool) {
	return defaultMapping(name)
}

func TestEscapedWithNamedMappings(t *testing.T) {
	result, err := SubstituteWithOptions("$${foo[bar]}", defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
	assert.NilError(t, err)
	assert.Check(t, is.Equal("${foo[bar]}", result))
}

func TestInvalidWithNamedMappings(t *testing.T) {
	invalidTemplates := []string{
		"${[]",
		"${[]}",
		"${ [ ] }",
		"${ foo[bar]}",
		"${foo[bar] }",
		"${foo [bar] }",
		"${foo [bar }",
		"${foo bar]}",
		"${$FOO[bar]}",       // Cannot use interpolation for named mapping's name
		"${FOO_${foo[bar]}}", // Cannot use named mapping for environment variable
		"${foo[bar]a:-def}",  // Cannot present characters (`a`) between closing bracket `]` and substitution function `:-`
	}

	for _, template := range invalidTemplates {
		_, err := SubstituteWithOptions(template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
		assert.ErrorContains(t, err, "Invalid template")
	}

	invalidMappings := NamedMappings{
		"FOO": func(name string) (string, bool) { return "invalid]", true },
	}
	testCases := []struct {
		template      string
		expectedError string
	}{
		{
			template:      "${FOO[~invalid]}",
			expectedError: `invalid key in named mapping: "~invalid"`,
		},
		{
			template:      "${FOO[${FOO[invalid]}]}",
			expectedError: `invalid key in named mapping: "${FOO[invalid]}" (resolved to "invalid]")`,
		},
		{
			template:      "${FOO[$$]}",
			expectedError: `invalid key in named mapping: "$$" (resolved to "$")`,
		},
		{
			template:      "${FOO[$$FOO]}",
			expectedError: `invalid key in named mapping: "$$FOO" (resolved to "$FOO")`,
		},
		{
			template:      "${FOO[$${FOO}]}",
			expectedError: `invalid key in named mapping: "$${FOO}" (resolved to "${FOO}")`,
		},
		{
			template:      "${FOO[${FOO}~invalid]}",
			expectedError: `invalid key in named mapping: "${FOO}~invalid" (resolved to "first~invalid")`,
		},
		{
			template:      "${FOO[${BAR[UNSET]}valid]}",
			expectedError: `named mapping not found: "BAR"`,
		},
	}
	for _, tc := range testCases {
		_, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(invalidMappings))
		assert.ErrorContains(t, err, tc.expectedError)
	}
}

func TestNonBracedWithNamedMappings(t *testing.T) {
	substituted, err := SubstituteWithOptions("$foo[bar]-bar", defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
	assert.NilError(t, err)
	assert.Equal(t, substituted, "[bar]-bar", "Named mappings bust be wrapped in brace to work. Expected [bar]-bar, got %s", substituted)
}

func TestNoValueNoDefaultWithNamedMappingsDisabled(t *testing.T) {
	// NOTE: Named mappings disabled, and `missing[foo]/FOO[BAR]` is not valid environment variable names
	// So they will be treated as literal strings and not be substituted
	for _, template := range []string{"This ${missing[foo]} var", "This ${FOO[BAR]} var"} {
		result, err := Substitute(template, defaultMapping)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(template, result))
	}
}

func TestNoValueNoDefaultWithNamedMappings(t *testing.T) {
	// NOTE: ${FOO[BAR]} will lookup named mapping `FOO`, instead of environment variable `FOO`
	// and ${missing[foo]} will lookup named mapping `missing`, which does not exist and error will be thrown
	for _, template := range []string{"This ${missing[foo]} var", "This ${FOO[BAR]} var"} {
		result, err := SubstituteWithOptions(template, defaultMapping, WithNamedMappings(NamedMappings{"FOO": envMapping}))
		if err != nil {
			assert.ErrorContains(t, err, fmt.Sprintf("named mapping not found: %q", "missing"))
			assert.ErrorType(t, err, reflect.TypeOf(&MissingNamedMappingError{}))
		} else {
			assert.Check(t, is.Equal("This  var", result))
		}
	}
}

func TestValueNoDefaultWithNamedMappings(t *testing.T) {
	// NOTE: In first case, $FOO still looks up environment variable `FOO`,
	// while in second case, ${FOO[FOO]} will lookup named mapping `FOO`, which reuses defaultMapping to continue looking up environment variable `FOO`
	for _, template := range []string{"This $FOO var", "This ${FOO[FOO]} var"} {
		result, err := SubstituteWithOptions(template, defaultMapping, WithNamedMappings(NamedMappings{"FOO": envMapping}))
		assert.NilError(t, err)
		assert.Check(t, is.Equal("This first var", result))
	}
}

func TestChainedSubstitutionFunctionWithNamedMappings(t *testing.T) {
	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			template: "ok ${env[BAR]:-def}", // EmptyValueWithSoftDefault
			expected: "ok def",
		},
		{
			template: "ok ${env[FOO]:-def}", // ValueWithSoftDefault
			expected: "ok first",
		},
		{
			template: "ok ${env[BAR]-def}", // EmptyValueWithHardDefault
			expected: "ok ",
		},
		{
			template: "ok ${env[UNSET_VAR]:+presence_value}", // PresentValueWithUnset
			expected: "ok ",
		},
		{
			template: "ok ${env[UNSET_VAR]+presence_value}", // PresentValueWithUnset2
			expected: "ok ",
		},
		{
			template: "ok ${env[FOO]:+presence_value}", // PresentValueWithNonEmpty
			expected: "ok presence_value",
		},
		{
			template: "ok ${env[FOO]+presence_value}", // PresentValueAndNonEmptyWithNonEmpty
			expected: "ok presence_value",
		},
		{
			template: "ok ${env[BAR]+presence_value}", // PresentValueWithSet
			expected: "ok presence_value",
		},
		{
			template: "ok ${env[BAR]:+presence_value}", // PresentValueAndNotEmptyWithSet
			expected: "ok ",
		},
		{
			template: "ok ${env[BAR]:-/non:-alphanumeric}", // NonAlphanumericDefault
			expected: "ok /non:-alphanumeric",
		},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("Named mapping should be able to be used with subsitution functions: %d", i), func(t *testing.T) {
			result, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
			assert.NilError(t, err)
			assert.Check(t, is.Equal(tc.expected, result))
		})
	}
}

func TestInterpolationExternalInterferenceWithNamedMappings(t *testing.T) {
	testCases := []struct {
		name     string
		template string
		expected string
	}{
		{
			template: "[ok] ${env[UNSET]}",
			expected: "[ok] ",
		},
		{
			template: "[ok] ${env[FOO]}",
			expected: "[ok] first",
		},
		{
			template: "[ok] ${env[FOO]} ${env[${BAR:-FOO}]}",
			expected: "[ok] first first", // Properly handled with `getFirstBraceClosingIndex`
		},
		{
			template: "aaa-${env[${env[${UNSET:-$FOO}]}]}?-:${env[${BAR:-FOO}]}}-${BAR:-test${UNSET:-$FOO}}:-bbb",
			expected: "aaa-?-:first}-testfirst:-bbb",
		},
		{
			template: "aaa-${env[${env[${UNSET:-$FOO}]}]}?-:${env[${BAR:-FOO}]}}-${BAR:-test${UNSET:-${env[FOO]}}}:-bbb", // The beginning `${env[...` and `...]}}}` will and shall be matched
			expected: "aaa-?-:first}-testfirst:-bbb",
		},
		{
			template: "aaa-${env[${env[${UNSET:-$FOO}]}]}?-:${env[${BAR:-FOO}]}}-${BAR:-test${UNSET:-${env[BAR]:-$FOO}}}:-bbb",
			expected: "aaa-?-:first}-testfirst:-bbb",
		},
		{
			template: "aaa-${env[${env[${UNSET:-$FOO}]}]}?-:${env[${BAR:-FOO}]}}-${BAR:-test${UNSET:-${env[BAR]:-${env[FOO]}}}}:-bbb",
			expected: "aaa-?-:first}-testfirst:-bbb",
		},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("Interpolation Should not be impacted by outer text: %d", i), func(t *testing.T) {
			result, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
			assert.NilError(t, err)
			assert.Check(t, is.Equal(tc.expected, result))
		})
	}
}

func TestDefaultsWithNestedExpansionWithNamedMappings(t *testing.T) {
	testCases := []struct {
		template string
		expected string
	}{
		{
			template: "ok ${env[${UNSET_VAR-FOO}]}",
			expected: "ok first",
		},
		{
			template: "ok ${UNSET_VAR-${env[FOO]}}",
			expected: "ok first",
		},
		{
			template: "ok ${env[UNSET_VAR]-${env[FOO]}}",
			expected: "ok first",
		},
		{
			template: "ok ${env[UNSET_VAR]-${FOO} ${FOO}}",
			expected: "ok first first",
		},
		{
			template: "ok ${env[UNSET_VAR]-${env[FOO]} ${env[FOO]}}",
			expected: "ok first first",
		},
		{
			template: "ok ${env[BAR]+$FOO}",
			expected: "ok first",
		},
		{
			template: "ok ${env[BAR]+${env[FOO]}}",
			expected: "ok first",
		},
		{
			template: "ok ${env[BAR]+${env[FOO]} ${env[FOO]:+second}}",
			expected: "ok first second",
		},
		{
			template: "ok ${env[${env[FOO]+BAR}]+${env[FOO]+${env[BAR]:-firstPresence}} ${env[FOO]:+second}}",
			expected: "ok firstPresence second",
		},
	}

	for _, tc := range testCases {
		result, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
		assert.NilError(t, err)
		assert.Check(t, is.Equal(tc.expected, result))
	}
}

func TestMandatoryVariableErrorsWithNamedMappings(t *testing.T) {
	testCases := []struct {
		template      string
		expectedError string
	}{
		{
			template:      "not ok ${env[UNSET_VAR]:?Mandatory Variable Unset}",
			expectedError: "required variable env[UNSET_VAR] is missing a value: Mandatory Variable Unset",
		},
		{
			template:      "not ok ${env[BAR]:?Mandatory Variable Empty}",
			expectedError: "required variable env[BAR] is missing a value: Mandatory Variable Empty",
		},
		{
			template:      "not ok ${env[UNSET_VAR]:?}",
			expectedError: "required variable env[UNSET_VAR] is missing a value",
		},
		{
			template:      "not ok ${env[UNSET_VAR]?Mandatory Variable Unset}",
			expectedError: "required variable env[UNSET_VAR] is missing a value: Mandatory Variable Unset",
		},
		{
			template:      "not ok ${env[UNSET_VAR]?}",
			expectedError: "required variable env[UNSET_VAR] is missing a value",
		},
	}

	for _, tc := range testCases {
		_, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
		assert.ErrorContains(t, err, tc.expectedError)
		assert.ErrorType(t, err, reflect.TypeOf(&MissingRequiredError{}))
	}
}

func TestMandatoryVariableErrorsWithNestedExpansionWithNamedMappings(t *testing.T) {
	testCases := []struct {
		template      string
		expectedError string
	}{
		{
			template:      "not ok ${env[UNSET_VAR]:?Mandatory Variable ${env[FOO]}}",
			expectedError: "required variable env[UNSET_VAR] is missing a value: Mandatory Variable first",
		},
		{
			template:      "not ok ${env[UNSET_VAR]?Mandatory Variable ${env[FOO]}}",
			expectedError: "required variable env[UNSET_VAR] is missing a value: Mandatory Variable first",
		},
	}

	for _, tc := range testCases {
		_, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
		assert.ErrorContains(t, err, tc.expectedError)
		assert.ErrorType(t, err, reflect.TypeOf(&MissingRequiredError{}))
	}
}

func TestDefaultsForMandatoryVariablesWithNamedMappings(t *testing.T) {
	testCases := []struct {
		template string
		expected string
	}{
		{
			template: "ok ${env[FOO]:?err}",
			expected: "ok first",
		},
		{
			template: "ok ${env[FOO]?err}",
			expected: "ok first",
		},
		{
			template: "ok ${env[BAR]?err}",
			expected: "ok ",
		},
	}

	for _, tc := range testCases {
		result, err := SubstituteWithOptions(tc.template, defaultMapping, WithNamedMappings(NamedMappings{"env": envMapping}))
		assert.NilError(t, err)
		assert.Check(t, is.Equal(tc.expected, result))
	}
}

func TestSubstituteWithCustomFuncWithNamedMappings(t *testing.T) {
	errIsMissing := func(substitution string, mapping Mapping) (string, bool, error) {
		var value string
		var found bool
		if strings.HasPrefix(substitution, "env[") {
			substitution = strings.TrimSuffix(strings.TrimPrefix(substitution, "env["), "]")
			value, found = envMapping(substitution)
		}
		if !found {
			value, found = mapping(substitution)
		}
		if !found {
			return "", true, &InvalidTemplateError{
				Template: fmt.Sprintf("required variable %s is missing a value", substitution),
			}
		}
		return value, true, nil
	}

	result, err := SubstituteWithOptions("ok ${env[FOO]}", defaultMapping,
		WithPattern(DefaultPattern), WithSubstitutionFunction(errIsMissing), WithNamedMappings(NamedMappings{"env": envMapping}))
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok first", result))

	result, err = SubstituteWith("ok ${BAR}", defaultMapping, DefaultPattern, errIsMissing)
	assert.NilError(t, err)
	assert.Check(t, is.Equal("ok ", result))

	_, err = SubstituteWith("ok ${NOTHERE}", defaultMapping, DefaultPattern, errIsMissing)
	assert.Check(t, is.ErrorContains(err, "required variable"))
}
