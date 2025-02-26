// copy_macos.go - macOS specific file copy
//
// (c) 2021 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build darwin

package fio

import (
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func sysCopyFile(dst, src string, perm fs.FileMode) error {
	err := unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW)
	if err == nil {
		return nil
	}

	if !errAny(err, syscall.ENOTSUP, syscall.ENOSYS) {
		return &CopyError{"clone", src, dst, err}
	}

	// fallback
	return slowCopy(dst, src, perm)
}

// macOS doesn't have the equiv fclonefile() that takes two fds.
// And clonefile(2) and fclonefileat(2) both require that the
// destination file NOT exist. So, we are stuck with slow path
func sysCopyFd(d, s *os.File) error {
	return copyViaMmap(d, s)
}
