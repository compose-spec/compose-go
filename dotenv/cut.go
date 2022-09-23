//go:build go1.18

package dotenv

import "bytes"

func bytesCut(s, sep []byte) (before, after []byte, found bool) {
	return bytes.Cut(s, sep)
}
