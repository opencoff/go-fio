// meta_nop.go -- metadata updates for unsupported systems
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

//go:build !unix

package fio

import (
	"fmt"
	"io/fs"
)

func clonetimes(dest string, fi *Info) error {
	return fmt.Errorf("utimes: not supported")
}

func mknod(dest string, src string, fi *Info) error {
	return fmt.Errorf("mknod: not supported")
}

// clone a symlink - ie we make the target point to the same one as src
func clonelink(dest string, src string, fi *Info) error {
	return fmt.Errorf("clonelink: not supported")
}
