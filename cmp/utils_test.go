// utils_test.go -- test harness utils
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.
package cmp_test

import (
	crand "crypto/rand"
	"flag"
	"fmt"
	"math/rand/v2"
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

	sz := 1024 + rand.Int64N(32768)
	b := make([]byte, sz)
	_, err = crand.Read(b)
	if err != nil {
		return fmt.Errorf("rand read: %w", err)
	}

	fd.Write(b)
	fd.Sync()
	return fd.Close()
}

var testDir = flag.String("testdir", "", "Use 'T' as the testdir for file I/O tests")

func getTmpdir(t *testing.T) string {
	assert := newAsserter(t)
	tmpdir := t.TempDir()

	if len(*testDir) > 0 {
		tmpdir = filepath.Join(*testDir, t.Name())
		err := os.MkdirAll(tmpdir, 0700)
		assert(err == nil, "mkdir %s: %s", tmpdir, err)
		t.Cleanup(func() {
			if t.Failed() {
				t.Logf("preserving %s ..\n", tmpdir)
			} else {
				os.RemoveAll(tmpdir)
			}
		})
	}
	return tmpdir
}
