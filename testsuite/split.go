// split.go -- split a string of the form key="a b c" into a tuple of
//  <key, [a, b, c]>

package main

import (
	"fmt"
	"github.com/opencoff/shlex"
	"strings"
)

// split a string of the form key="a b c" and return
//
//	<key, []{a, b, c}>
func Split(s string) (string, []string, error) {
	i := strings.Index(s, "=")
	if i < 0 {
		return "", nil, fmt.Errorf("%s: missing separator '='", s)
	}

	key := strings.ToLower(s[:i])

	val, err := shlex.Split(strings.TrimSpace(s[i+1:]))
	return key, val, err
}
