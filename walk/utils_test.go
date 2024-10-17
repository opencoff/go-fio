package walk

import (
	"fmt"
	"runtime"
	"testing"
)

func newAsserter(t *testing.T) func(cond bool, msg string, args ...interface{}) {
	return func(cond bool, msg string, args ...interface{}) {
		if cond {
			return
		}

		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "???"
			line = 0
		}

		s := fmt.Sprintf(msg, args...)
		t.Fatalf("\n%s: %d: Assertion failed: %s\n", file, line, s)
	}
}

func newBenchAsserter(b *testing.B) func(cond bool, msg string, args ...interface{}) {
	return func(cond bool, msg string, args ...interface{}) {
		if cond {
			return
		}

		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "???"
			line = 0
		}

		s := fmt.Sprintf(msg, args...)
		b.Errorf("\n%s: %d: Assertion failed: %s\n", file, line, s)
	}
}
