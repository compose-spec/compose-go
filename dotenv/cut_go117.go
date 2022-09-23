//go:build !go1.18
// +build !go1.18

package dotenv

import "bytes"

func bytesCut(s, sep []byte) (before, after []byte, found bool) {
	if i := bytes.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, nil, false
}
