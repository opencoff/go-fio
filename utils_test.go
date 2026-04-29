// utils_test.go -- shared test helpers for the fio package
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim is made to its
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

func mkfilex(fn string) error {
	bn := filepath.Dir(fn)
	if err := os.MkdirAll(bn, 0700); err != nil {
		return fmt.Errorf("mkdir: %s: %w", bn, err)
	}

	fd, err := os.OpenFile(fn, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("creat: %s: %w", fn, err)
	}

	_, _ = fd.Write([]byte("hello"))
	_ = fd.Sync()
	return fd.Close()
}
