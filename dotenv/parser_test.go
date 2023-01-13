package dotenv

import (
	"testing"

	"gotest.tools/v3/assert"
)

var testInput = `
a=b
a[1]=c
a.propertyKey=d
`

func TestParseBytes(t *testing.T) {
	p := newParser()

	var inputBytes = []byte(testInput)
	expectedOutput := map[string]string{
		"a":             "b",
		"a[1]":          "c",
		"a.propertyKey": "d",
	}

	out := map[string]string{}
	err := p.parseBytes([]byte(inputBytes), out, nil)

	assert.NilError(t, err)
	assert.Equal(t, len(expectedOutput), len(out))
	for key, value := range expectedOutput {
		assert.Equal(t, value, out[key])
	}
}
