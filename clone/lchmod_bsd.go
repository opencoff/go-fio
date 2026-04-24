// lchmod_bsd.go -- lchmod via fchmodat(AT_SYMLINK_NOFOLLOW) on BSDs and darwin
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

package clone

import (
	"errors"
	"io/fs"
	"syscall"

	"golang.org/x/sys/unix"
)

// lchmod applies mode to a symlink without following it.
// darwin, FreeBSD, OpenBSD, and NetBSD all support
// fchmodat(AT_FDCWD, path, mode, AT_SYMLINK_NOFOLLOW). If the kernel
// or filesystem surprises us with ENOTSUP/EOPNOTSUPP, swallow it so
// clones do not fail over this metadata nicety.
func lchmod(path string, mode fs.FileMode) error {
	err := unix.Fchmodat(unix.AT_FDCWD, path, uint32(mode.Perm()), unix.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) {
		return nil
	}
	return err
}
