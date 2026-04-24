// testutil.go -- cross-platform test helpers for go-fio.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

// Package testutil provides tiny helpers for go-fio's privileged
// tests. The convention:
//
//   - Tests that require root live in files that self-skip via
//     testutil.RequirePrivileged(t) at the top.
//   - Linux-specific tests live in *_linux_test.go files; Go's
//     filename build gating excludes them on other platforms.
//   - No //go:build privileged tag. The full test binary is always
//     produced; privileged tests show up as "skipped" under plain
//     `go test -v` and run under `sudo go test -v`.
//
// The package is under internal/ so external consumers of go-fio
// cannot depend on these helpers.
package testutil

import (
	"os"
	"testing"
)

// RequirePrivileged skips the calling test unless the process is
// running as effective uid 0. Uses t.Skip (which calls
// runtime.Goexit) so the caller need not check a return value.
//
// Typical use:
//
//	func TestThatNeedsRoot(t *testing.T) {
//	    testutil.RequirePrivileged(t)
//	    // ... test body; only runs when root ...
//	}
func RequirePrivileged(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skipf("needs root to test %s; skipping", t.Name())
	}
}
