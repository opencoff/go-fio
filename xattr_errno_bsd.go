// xattr_errno_bsd.go -- "xattr not found" predicate for darwin/BSD
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

//go:build darwin || freebsd || openbsd || netbsd

package fio

import (
	"errors"
	"syscall"
)

// errXattrNotFound is the canonical platform errno that indicates a
// named extended attribute does not exist. On darwin and BSDs this
// is ENOATTR.
var errXattrNotFound = syscall.ENOATTR

// isXattrNotFound reports whether err is the platform's "xattr not
// found" errno. On darwin/BSD that is ENOATTR; the kernel returns
// it for getxattr/removexattr on a key that does not exist (or was
// removed between a listxattr and the subsequent access).
func isXattrNotFound(err error) bool {
	return errors.Is(err, syscall.ENOATTR)
}
