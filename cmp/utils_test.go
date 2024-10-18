// utils_test.go -- test harness utilities
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
package fio

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

type rootdir string

func (d rootdir) mkfile(nm string) error {
	fn := filepath.Join(string(d), nm)
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

func (d rootdir) mkdir(nm string) error {
	fn := filepath.Join(string(d), nm)
	if err := os.MkdirAll(fn, 0700); err != nil {
		return fmt.Errorf("mkdir: %s: %w", fn, err)
	}
	return nil
}

func (d rootdir) symlink(origin, linkname string) error {
	src := filepath.Join(string(d), origin)
	dst := filepath.Join(string(d), linkname)

	dn := filepath.Dir(dst)
	if err := os.MkdirAll(dn, 0700); err != nil {
		return fmt.Errorf("symlink: mkdir %s: %w", dn, err)
	}

	if err := os.Symlink(dst, src); err != nil {
		return fmt.Errorf("symlink: %s %s: %w", src, dst, err)
	}
	return nil
}
