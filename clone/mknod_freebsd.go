// mknod_freebsd.go -- mknod(2) for FreeBSD.
//
// SPDX-License-Identifier: GPL-2.0
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

//go:build freebsd

package clone

import (
	"syscall"

	"github.com/opencoff/go-fio"
)

// mknod creates a special file at dst matching fi. Same contract as
// the linux/darwin variant; the only platform delta is that FreeBSD's
// syscall.Mknod takes dev as uint64 rather than int.
func mknod(dst string, fi *fio.Info) error {
	if err := syscall.Mknod(dst, sysMode(fi.Mode()), fi.Rdev); err != nil {
		return &Error{"mknod", fi.Path(), dst, err}
	}
	return nil
}
