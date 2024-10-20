// panicf.go -- panic with fmt

package main

import (
	"fmt"
	"os"
)

func panicf(s string, v ...interface{}) {
	z := fmt.Sprintf("%s: %s", os.Args[0], s)
	m := fmt.Sprintf(z, v...)
	if n := len(m); m[n-1] != '\n' {
		m += "\n"
	}
	panic(m)
}
