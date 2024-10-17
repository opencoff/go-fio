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
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func chown(dest string, _ string, fi fs.FileInfo) error {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if err := syscall.Chown(dest, int(st.Uid), int(st.Gid)); err != nil {
			return fmt.Errorf("chown: %w", err)
		}
	}
	return nil
}

func chmod(dest string, _ string, fi fs.FileInfo) error {
	return os.Chmod(dest, fi.Mode())
}

// clone a symlink - ie we make the target point to the same one as src
func clonelink(dest string, src string, fi fs.FileInfo) error {
	targ, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("readlink: %w", err)
	}
	if err = os.Symlink(targ, dest); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	if err := utimes(dest, src, fi); err != nil {
		return err
	}
	return lclonexattr(dest, src, fi)
}

func clonexattr(dest, src string, _ fs.FileInfo) error {
	x, err := GetXattr(src)
	if err != nil {
		return err
	}

	return ReplaceXattr(dest, x)
}

// clone the xattr of the symlink itself
func lclonexattr(dest, src string, _ fs.FileInfo) error {
	x, err := LgetXattr(src)
	if err != nil {
		return err
	}

	return LreplaceXattr(dest, x)
}
