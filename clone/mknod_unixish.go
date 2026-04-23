// mknod_freebsd.go -- mknod(2) for freebsd
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

//go:build linux || darwin

package clone

import (
	"io/fs"
	"syscall"

	"github.com/opencoff/go-fio"
)

func mknod(dst string, fi *fio.Info) error {
	if err := syscall.Mknod(dst, sysMode(fi.Mode()), int(fi.Rdev)); err != nil {
		return &Error{"mknod", fi.Path(), dst, err}
	}
	return nil
}

// sysMode converts a Go fs.FileMode into a POSIX-style mode bitmask
// suitable for syscall.Mknod. fs.FileMode carries the file-type in the
// high bits (ModeDevice, ModeCharDevice, ModeNamedPipe, ModeSocket) and
// the permission bits in the low 9 bits; the kernel expects the type
// encoded via S_IFCHR/S_IFBLK/S_IFIFO/S_IFSOCK in the low bits along
// with the permissions.
func sysMode(m fs.FileMode) uint32 {
	out := uint32(m.Perm())
	switch {
	case m&fs.ModeDevice != 0 && m&fs.ModeCharDevice != 0:
		out |= syscall.S_IFCHR
	case m&fs.ModeDevice != 0:
		out |= syscall.S_IFBLK
	case m&fs.ModeNamedPipe != 0:
		out |= syscall.S_IFIFO
	case m&fs.ModeSocket != 0:
		out |= syscall.S_IFSOCK
	}
	return out
}
