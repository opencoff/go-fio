// copyfile.go - copy a file efficiently using platform specific
// primitives and fallback to simple mmap'd copy.
//
// (c) 2024 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

package fio

import (
	"io/fs"
	"os"
)

// CopyFile copies files 'src' to 'dst' using the most efficient OS primitive
// available on the runtime platform. CopyFile will use copy-on-write
// facilities if the underlying file-system implements it. It will
// fallback to copying via memory mapping 'src' and writing the blocks
// to 'dst'.
func CopyFile(dst, src string, perm fs.FileMode) error {
	// never overwrite an existing file.
	_, err := Stat(dst)
	if err == nil {
		return &CopyError{"stat-dst", src, dst, err}
	}

	s, err := os.Open(src)
	if err != nil {
		return &CopyError{"open-src", src, dst, err}
	}

	defer s.Close()

	d, err := NewSafeFile(dst, OPT_OVERWRITE, os.O_CREATE|os.O_RDWR|os.O_EXCL, perm)
	if err != nil {
		return &CopyError{"safefile", src, dst, err}
	}

	defer d.Abort()

	if err = copyFile(d.File, s); err != nil {
		return err
	}
	if err = d.Close(); err != nil {
		return &CopyError{"close", src, dst, err}
	}

	return nil
}

// CopyFd copies open files 'src' to 'dst' using the most efficient OS
// primitive available on the runtime platform. CopyFile will use
// copy-on-write facailities if the underlying file-system implements it.
// It will fallback to copying via memory mapping 'src' and writing the
// blocks to 'dst'.
func CopyFd(dst, src *os.File) error {
	if err := copyFile(dst, src); err != nil {
		return err
	}

	if err := dst.Sync(); err != nil {
		return &CopyError{"sync-dst", src.Name(), dst.Name(), err}
	}
	return nil
}

// copyFile copies using the best os primitive when possible
// and falls back to mmap based copies
func copyFile(dst, src *os.File) error {
	di, err := Lstat(dst.Name())
	if err != nil {
		return &CopyError{"lstat-src", src.Name(), dst.Name(), err}
	}

	si, err := Lstat(src.Name())
	if err != nil {
		return &CopyError{"lstat-dst", src.Name(), dst.Name(), err}
	}

	if di.IsSameFS(si) {
		return sys_copyFile(dst, src)
	}

	return copyViaMmap(dst, src)
}
