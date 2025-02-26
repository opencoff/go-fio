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
	return sysCopyFile(dst, src, perm)
}

// CopyFd copies open files 'src' to 'dst' using the most efficient OS
// primitive available on the runtime platform. CopyFile will use
// copy-on-write facailities if the underlying file-system implements it.
// It will fallback to copying via memory mapping 'src' and writing the
// blocks to 'dst'.
func CopyFd(dst, src *os.File) error {
	return sysCopyFd(dst, src)
}
