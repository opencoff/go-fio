// mknod_unix.go -- mknod(2) wrapper for linux and darwin.
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

//go:build linux || darwin

package clone

import (
	"syscall"

	"github.com/opencoff/go-fio"
)

// mknod creates a special file at dst matching fi. Uses fi.Rdev (the
// device this special file represents) - not fi.Dev (the filesystem
// hosting the source) - and translates Go's fs.FileMode type bits
// into POSIX S_IF* bits via sysMode().
func mknod(dst string, fi *fio.Info) error {
	if err := syscall.Mknod(dst, sysMode(fi.Mode()), int(fi.Rdev)); err != nil { // #nosec G115 -- Mknod takes int; real device IDs fit
		return &Error{"mknod", fi.Path(), dst, err}
	}
	return nil
}
