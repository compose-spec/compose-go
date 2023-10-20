package dotenv

import (
	"testing"

	"github.com/compose-spec/compose-go/v2/template"
	"github.com/stretchr/testify/assert"
)

var envMap = map[string]string{
	// UNSET_VAR: <Cannot be here :D>
	"EMPTY_VAR": "",
	"TEST_VAR":  "Test Value",
}

var notFoundLookup = func(s string) (string, bool) {
	return "", false
}

func TestExpandIfEmptyOrUnset(t *testing.T) {
	templateResults := []struct {
		name   string
		input  string
		result string
	}{
		{
			"Expand if empty or unset: UNSET_VAR",
			"RESULT=${UNSET_VAR:-Default Value}",
			"RESULT=Default Value",
		},
		{
			"Expand if empty or unset: EMPTY_VAR",
			"RESULT=${EMPTY_VAR:-Default Value}",
			"RESULT=Default Value",
		},
		{
			"Expand if empty or unset: TEST_VAR",
			"RESULT=${TEST_VAR:-Default Value}",
			"RESULT=Test Value",
		},
	}

	for _, expected := range templateResults {
		t.Run(expected.name, func(t *testing.T) {
			result, err := expandVariables(expected.input, envMap, notFoundLookup)
			assert.Nil(t, err)
			assert.Equal(t, result, expected.result)
		})
	}
}

func TestExpandIfUnset(t *testing.T) {
	templateResults := []struct {
		name   string
		input  string
		result string
	}{
		{
			"Expand if unset: UNSET_VAR",
			"RESULT=${UNSET_VAR-Default Value}",
			"RESULT=Default Value",
		},
		{
			"Expand if unset: EMPTY_VAR",
			"RESULT=${EMPTY_VAR-Default Value}",
			"RESULT=",
		},
		{
			"Expand if unset: TEST_VAR",
			"RESULT=${TEST_VAR-Default Value}",
			"RESULT=Test Value",
		},
	}

	for _, expected := range templateResults {
		t.Run(expected.name, func(t *testing.T) {
			result, err := expandVariables(expected.input, envMap, notFoundLookup)
			assert.Nil(t, err)
			assert.Equal(t, result, expected.result)
		})
	}
}

func TestErrorIfEmptyOrUnset(t *testing.T) {
	templateResults := []struct {
		name   string
		input  string
		result string
		err    error
	}{
		{
			"Error empty or unset: UNSET_VAR",
			"RESULT=${UNSET_VAR:?Test error}",
			"RESULT=${UNSET_VAR:?Test error}",
			&template.MissingRequiredError{Variable: "UNSET_VAR", Reason: "Test error"},
		},
		{
			"Error empty or unset: EMPTY_VAR",
			"RESULT=${EMPTY_VAR:?Test error}",
			"RESULT=${EMPTY_VAR:?Test error}",
			&template.MissingRequiredError{Variable: "EMPTY_VAR", Reason: "Test error"},
		},
		{
			"Error empty or unset: TEST_VAR",
			"RESULT=${TEST_VAR:?Default Value}",
			"RESULT=Test Value",
			nil,
		},
	}

	for _, expected := range templateResults {
		t.Run(expected.name, func(t *testing.T) {
			result, err := expandVariables(expected.input, envMap, notFoundLookup)
			assert.Equal(t, expected.err, err)
			assert.Equal(t, expected.result, result)
		})
	}
}

func TestErrorIfUnset(t *testing.T) {
	templateResults := []struct {
		name   string
		input  string
		result string
		err    error
	}{
		{
			"Error on unset: UNSET_VAR",
			"RESULT=${UNSET_VAR?Test error}",
			"RESULT=${UNSET_VAR?Test error}",
			&template.MissingRequiredError{Variable: "UNSET_VAR", Reason: "Test error"},
		},
		{
			"Error on unset: EMPTY_VAR",
			"RESULT=${EMPTY_VAR?Test error}",
			"RESULT=",
			nil,
		},
		{
			"Error on unset: TEST_VAR",
			"RESULT=${TEST_VAR?Default Value}",
			"RESULT=Test Value",
			nil,
		},
	}

	for _, expected := range templateResults {
		t.Run(expected.name, func(t *testing.T) {
			result, err := expandVariables(expected.input, envMap, notFoundLookup)
			assert.Equal(t, expected.err, err)
			assert.Equal(t, expected.result, result)
		})
	}
}
