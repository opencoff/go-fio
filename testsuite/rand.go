// rand.go - handy random bytes/ints collection

package main

import (
	"crypto/rand"
	"fmt"
	"golang.org/x/exp/constraints"
)

// fill buffer 'buf' with random bytes
func randBytes(buf []byte) {
	n, err := rand.Read(buf)
	if err != nil {
		s := fmt.Sprintf("rand: can't read %d bytes: %s", len(buf), err)
		panic(s)
	}
	if n != len(buf) {
		s := fmt.Sprintf("rand: partial read: expected %d, read %d bytes", len(buf), n)
		panic(s)
	}
}

// make a new buffer of 'n' bytes and fill it with
// random bytes
func randBuf[T constraints.Integer](n T) []byte {
	b := make([]byte, n)
	randBytes(b)
	return b
}
