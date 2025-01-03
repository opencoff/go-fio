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
	return nil
}
