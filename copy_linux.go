// copy_linux.go - Linux specific file copy
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

//go:build linux

package fio

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// Do copies in chunks of _ioChunkSize
const _ioChunkSize int = 256 * 1024

// optimized copy for linux and safe fallback to mmap
func sysCopyFile(dst, src string, perm fs.FileMode) error {
	// never overwrite an existing file.
	si, err := Stat(dst)
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

	// we have to wait until the safe-file is created before we
	// can check if it's on the same FS
	di, err := Fstat(d.File)
	if err != nil {
		return &CopyError{"fstat-dst", src, dst, err}
	}

	switch di.IsSameFS(si) {
	case true:
		err = sysCopyFd(d.File, s)
	case false:
		err = copyViaMmap(d.File, s)
	}

	if err != nil {
		return err
	}

	if err = d.Close(); err != nil {
		return &CopyError{"close", src, dst, err}
	}

	return nil
}

// try to use reflinks for copying where possible.
// Fallback to copy_file_range(2) which is available on all linuxes.
func sysCopyFd(dst, src *os.File) error {
	d := int(dst.Fd())
	s := int(src.Fd())

	// First try to reflink.
	err := unix.IoctlFileClone(int(d), int(s))
	if err == nil {
		return nil
	}
	if !errAny(err, syscall.ENOTSUP, syscall.ENOSYS, syscall.EXDEV) {
		return &CopyError{"clone", src.Name(), dst.Name(), err}
	}

	st, err := src.Stat()
	if err != nil {
		return &CopyError{"stat-src", src.Name(), dst.Name(), err}
	}

	// Fallback to copy_file_range(2)
	var roff, woff int64
	sz := st.Size()
	for sz > 0 {
		n := min(_ioChunkSize, int(sz))
		m, err := unix.CopyFileRange(s, &roff, d, &woff, n, 0)
		if err != nil {
			return &CopyError{"copy_file_range", src.Name(), dst.Name(), err}
		}
		if m == 0 {
			return &CopyError{"copy_file_range", src.Name(), dst.Name(),
				fmt.Errorf("zero sized transfer at off %d", roff)}
		}
		sz -= int64(m)
		roff += int64(m)
		woff += int64(m)
	}

	if _, err = dst.Seek(0, os.SEEK_SET); err != nil {
		return &CopyError{"seek", src.Name(), dst.Name(), err}
	}

	return nil
}
