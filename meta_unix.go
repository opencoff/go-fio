// meta_unix.go -- set file times for unixish platforms
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

//go:build unix

package fio

import (
	"os"
)

// clone a symlink - ie we make the target point to the same one as src
func clonelink(dst string, src string, fi *Info) error {
	targ, err := os.Readlink(src)
	if err != nil {
		return &CloneError{"readlink", src, dst, err}
	}
	if err = os.Symlink(targ, dst); err != nil {
		return &CloneError{"symlink", src, dst, err}
	}

	return nil
}
