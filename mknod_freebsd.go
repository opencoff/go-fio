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

//go:build freebsd

package fio

import (
	"fmt"
	"io/fs"
	"syscall"
)

func mknod(dest string, src string, fi *Info) error {
	if err := syscall.Mknod(dest, uint32(fi.Mode()), fi.Dev); err != nil {
		return fmt.Errorf("mknod: %w", err)
	}
	if err := utimes(dest, src, fi); err != nil {
		return err
	}
	return clonexattr(dest, src, fi)
}
