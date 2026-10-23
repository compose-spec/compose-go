package dotenv

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// FuzzUnmarshal locks the safety of the .env parser, with variable expansion
// on: whatever the file, parsing returns a map or an error, never panics, and
// is deterministic, with or without a lookup function. The variables of a
// file that parses survive a round trip: written back as single-quoted
// literals (which are neither escaped nor expanded), the file parses to the
// same variables.
func FuzzUnmarshal(f *testing.F) {
	for _, seed := range []string{
		"",
		"\n\n# comment only\n",
		"A=b",
		"A = b",
		"A: b",
		"export A=b",
		"A=b # inline comment",
		"A='single $quoted'",
		`A="double \"quoted\" \n\t\$x"`,
		"A=\"multi\nline\"",
		"A=${B:-default}\nB=${A}",
		"A=${UNSET:?required}",
		"A=$$escaped",
		`A=\$escaped`,
		"INHERITED",
		"A=1\nINHERITED\nB=2",
		"A=\"unterminated",
		"A='unterminated",
		"=value",
		"A B=c",
		"A\x00=b",
		"\xef\xbb\xbfA=b",
		"A=b\r\nC=d\r\n",
		"árvíztűrő-TÜKÖRFÚRÓGÉP=ÁRVÍZTŰRŐ",
		"A=\"\\0123 \\0 \\777\"",
		"A=${B:-${C:-${D}}}",
		"A=${FROM_ENV:+${UNSET:?required}}",
		"a[1]=c\na.propertyKey=d",
	} {
		f.Add(seed)
	}
	// the fixtures of the parser tests are as good a corpus as any
	fixtures, _ := filepath.Glob(filepath.Join("fixtures", "*.env"))
	for _, name := range fixtures {
		if data, err := os.ReadFile(name); err == nil {
			f.Add(string(data))
		}
	}

	lookup := func(key string) (string, bool) {
		if key == "FROM_ENV" {
			return "env", true
		}
		return "", false
	}

	f.Fuzz(func(t *testing.T, src string) {
		got, err := UnmarshalWithLookup(src, nil)
		again, err2 := UnmarshalWithLookup(src, nil)
		if !reflect.DeepEqual(got, again) || (err == nil) != (err2 == nil) {
			t.Fatalf("UnmarshalWithLookup(%q) is not deterministic: (%v, %v) then (%v, %v)", src, got, err, again, err2)
		}

		withLookup, errLookup := UnmarshalWithLookup(src, lookup)
		againLookup, errAgainLookup := UnmarshalWithLookup(src, lookup)
		if !reflect.DeepEqual(withLookup, againLookup) || (errLookup == nil) != (errAgainLookup == nil) {
			t.Fatalf("UnmarshalWithLookup(%q) with a lookup function is not deterministic: (%v, %v) then (%v, %v)", src, withLookup, errLookup, againLookup, errAgainLookup)
		}

		if err != nil {
			return
		}
		var out strings.Builder
		for key, value := range got {
			if strings.ContainsRune(value, '\'') || strings.HasSuffix(value, `\`) {
				// not representable as a single-quoted literal
				return
			}
			out.WriteString(key + "='" + value + "'\n")
		}
		back, err := UnmarshalWithLookup(out.String(), nil)
		if err != nil || !reflect.DeepEqual(got, back) {
			t.Fatalf("round trip of %q through %q = (%v, %v), want %v", src, out.String(), back, err, got)
		}
	})
}
