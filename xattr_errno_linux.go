// xattr_errno_linux.go -- "xattr not found" predicate for linux
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

//go:build linux

package fio

import (
	"errors"
	"syscall"
)

// errXattrNotFound is the canonical platform errno that indicates a
// named extended attribute does not exist. Exported into tests via
// this package-level so the cross-platform test file can reference
// it without syscall-switching.
var errXattrNotFound = syscall.ENODATA

// isXattrNotFound reports whether err is the platform's "xattr not
// found" errno. On Linux that is ENODATA; the kernel returns it for
// getxattr/removexattr on a key that does not exist (or was removed
// between a listxattr and the subsequent access).
func isXattrNotFound(err error) bool {
	return errors.Is(err, syscall.ENODATA)
}
