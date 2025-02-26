// copy_mmap.go - copy using mmap(2)
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

package fio

import (
	"io/fs"
	"os"

	"github.com/opencoff/go-mmap"
)

// Use mmap(2) to copy src to dst.
func copyViaMmap(dst, src *os.File) error {
	_, err := mmap.Reader(src, func(b []byte) error {
		_, err := fullWrite(dst, b)
		return err
	})
	if err != nil {
		return &CopyError{"mmap-reader", src.Name(), dst.Name(), err}
	}
	_, err = dst.Seek(0, os.SEEK_SET)
	if err != nil {
		return &CopyError{"seek-mmap", src.Name(), dst.Name(), err}
	}

	if err = dst.Sync(); err != nil {
		return &CopyError{"dst-sync", src.Name(), dst.Name(), err}
	}
	return nil
}

// slowCopy copies src to dst via mmap
func slowCopy(dst, src string, perm fs.FileMode) error {
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

	if err = copyViaMmap(d.File, s); err != nil {
		return err
	}

	if err = d.Close(); err != nil {
		return &CopyError{"close", src, dst, err}
	}

	return nil
}
