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
	"fmt"
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
		return fmt.Errorf("copyfile: destination %s already exists", dst)
	}

	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("copyfile: %w", err)
	}

	defer s.Close()

	d, err := NewSafeFile(dst, OPT_COW, os.O_CREATE|os.O_RDWR|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("copyfile: %w", err)
	}

	defer d.Abort()
	if err = copyFile(d.File, s); err == nil {
		err = d.Close()
	}

	if err != nil {
		return fmt.Errorf("copyfile: %w", err)
	}
	return nil
}

// CopyFd copies open files 'src' to 'dst' using the most efficient OS
// primitive available on the runtime platform. CopyFile will use
// copy-on-write facailities if the underlying file-system implements it.
// It will fallback to copying via memory mapping 'src' and writing the
// blocks to 'dst'.
func CopyFd(dst, src *os.File) error {
	err := copyFile(dst, src)
	if err == nil {
		err = dst.Sync()
	}
	return err
}
