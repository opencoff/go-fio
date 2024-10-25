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

package fio

import (
	"syscall"
)

func mknod(dst string, fi *Info) error {
	if err := syscall.Mknod(dst, uint32(fi.Mode()), int(fi.Dev)); err != nil {
		return &CloneError{"mknod", fi.Name(), dst, err}
	}
	return nil
}
