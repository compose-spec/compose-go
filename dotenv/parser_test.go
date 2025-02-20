package dotenv

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

var testInput = `
a=b
a[1]=c
a.propertyKey=d
árvíztűrő-TÜKÖRFÚRÓGÉP=ÁRVÍZTŰRŐ-tükörfúrógép
`

func TestParseBytes(t *testing.T) {
	p := newParser()

	expectedOutput := map[string]string{
		"a":                      "b",
		"a[1]":                   "c",
		"a.propertyKey":          "d",
		"árvíztűrő-TÜKÖRFÚRÓGÉP": "ÁRVÍZTŰRŐ-tükörfúrógép",
	}

	out := map[string]string{}
	err := p.parse(testInput, out, nil)

	assert.NilError(t, err)
	assert.Equal(t, len(expectedOutput), len(out))
	for key, value := range expectedOutput {
		assert.Equal(t, value, out[key])
	}
}

func TestParseVariable(t *testing.T) {
	err := newParser().parse("%!(EXTRA string)=foo", map[string]string{}, nil)
	assert.Error(t, err, "line 1: unexpected character \"%\" in variable name \"%!(EXTRA string)=foo\"")
}

func TestMemoryExplosion(t *testing.T) {
	p := newParser()
	var startMemStats runtime.MemStats
	var endMemStats runtime.MemStats
	runtime.ReadMemStats(&startMemStats)

	size := 1000
	input := []string{}
	for i := 0; i < size; i++ {
		input = append(input, fmt.Sprintf("KEY%d=VALUE%d", i, i))
	}
	out := map[string]string{}
	err := p.parse(strings.Join(input, "\n"), out, nil)
	assert.NilError(t, err)
	assert.Equal(t, size, len(out))
	runtime.ReadMemStats(&endMemStats)
	assert.Assert(t, endMemStats.Alloc-startMemStats.Alloc < uint64(size)*1000, /* assume 1K per line */
		"memory usage should be linear with input size. Memory grew by: %d",
		endMemStats.Alloc-startMemStats.Alloc)
}
