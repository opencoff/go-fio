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
	"syscall"

	"github.com/opencoff/go-fio"
)

func mknod(dst string, fi *fio.Info) error {
	if err := syscall.Mknod(dst, uint32(fi.Mode()), int(fi.Dev)); err != nil {
		return &Error{"mknod", fi.Path(), dst, err}
	}
	return nil
}
