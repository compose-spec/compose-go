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

// A variable without value takes its value from the environment, also when it
// is the last line of a file that has no final line break.
func TestParseInheritedAtEndOfFile(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "INHERITED" {
			return "from env", true
		}
		return "", false
	}
	for name, tc := range map[string]struct {
		src  string
		want map[string]string
	}{
		"only line":          {"INHERITED", map[string]string{"INHERITED": "from env"}},
		"trailing space":     {"INHERITED  ", map[string]string{"INHERITED": "from env"}},
		"after a valid line": {"A=1\nINHERITED", map[string]string{"A": "1", "INHERITED": "from env"}},
		"windows line break": {"A=1\r\nINHERITED\r", map[string]string{"A": "1", "INHERITED": "from env"}},
		"unset":              {"A=1\nUNSET", map[string]string{"A": "1"}},
	} {
		t.Run(name, func(t *testing.T) {
			out := map[string]string{}
			err := newParser().parse(tc.src, out, lookup)
			assert.NilError(t, err)
			assert.DeepEqual(t, out, tc.want)
		})
	}
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
