// lchmod_linux.go -- lchmod stub for linux
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

//go:build linux

package clone

import "io/fs"

// lchmod applies mode to a symlink without following it. Linux has
// no such syscall: fchmodat(AT_SYMLINK_NOFOLLOW) returns EOPNOTSUPP
// for symlinks and there is no other way. We silently succeed -
// symlink mode bits are not consulted by the Linux kernel anyway,
// so the clone is functionally complete without this metadata.
func lchmod(_ string, _ fs.FileMode) error {
	return nil
}
