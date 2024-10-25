package clone

import (
	"fmt"
	"os"
	"path/filepath"
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

func mkfilex(fn string) error {
	bn := filepath.Dir(fn)
	if err := os.MkdirAll(bn, 0700); err != nil {
		return fmt.Errorf("mkdir: %s: %w", bn, err)
	}

	fd, err := os.OpenFile(fn, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("creat: %s: %w", fn, err)
	}

	fd.Write([]byte("hello"))
	fd.Sync()
	return fd.Close()
}
