// clone.go - clone a file entry (file|dir|special)
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
	"path/filepath"
)

type op func(dest, src string, fi *Info) error

// order of applying these is important; we can't update
// certain attributes if we're not the owner. So, we have
// to do it in the end.
var _Mdupdaters = []op{
	clonexattr,
	chmod,
	chown,
	utimes,
}

// update all the metadata
func updateMeta(dest, src string, fi *Info) error {
	for _, fp := range _Mdupdaters {
		if err := fp(dest, src, fi); err != nil {
			return err
		}
	}
	return nil
}

// CloneMetadata clones all the metadata from src to dst: the metadata
// is atime, mtime, uid, gid, mode/perm, xattr
func CloneMetadata(dst, src string) error {
	fi, err := Lstat(src)
	if err == nil {
		err = updateMeta(dst, src, fi)
	}

	if err != nil {
		return fmt.Errorf("clonemeta: %w", err)
	}
	return nil
}

// UpdateMetadata writes new metadata of 'dst' from 'fi'
func UpdateMetadata(dst string, fi *Info) error {
	if err := updateMeta(dst, dst, fi); err != nil {
		return fmt.Errorf("clonemeta: %w", err)
	}
	return nil
}

// CloneFile copies src to dst - including all copyable file attributes
// and xattr. CloneFile will use the best available CoW facilities provided
// by the OS and Filesystem. It will fall back to using copy via mmap(2) on
// systems that don't have CoW semantics.
func CloneFile(dst, src string) error {
	// never overwrite an existing file.
	_, err := Lstat(dst)
	if err == nil {
		return fmt.Errorf("clonefile: destination %s already exists", dst)
	}

	fi, err := Lstat(src)
	if err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	defer s.Close()

	mode := fi.Mode()
	if mode.IsRegular() {
		return copyRegular(dst, s, fi)
	}

	switch mode.Type() {
	case fs.ModeDir:
		if err = os.MkdirAll(dst, mode&fs.ModePerm|0100); err != nil {
			return err
		}

		// update metadata; caller is responsible for deep clone of
		// a directory.
		err = updateMeta(dst, src, fi)

	case fs.ModeSymlink:
		err = clonelink(dst, src, fi)

	case fs.ModeDevice, fs.ModeNamedPipe:
		err = mknod(dst, src, fi)

	//case ModeSocket: XXX Add named socket support

	default:
		err = fmt.Errorf("clonefile: %s: unsupported type %#x", src, mode)
	}

	return err
}

// copy a regular file to another regular file
func copyRegular(dst string, s *os.File, fi *Info) error {
	// make the intermediate dirs of the dest
	dn := filepath.Dir(dst)
	if err := os.MkdirAll(dn, 0100|fs.ModePerm&fi.Mode()); err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	// We create the file so that we can write to it; we'll update the perm bits
	// later on
	d, err := NewSafeFile(dst, OPT_COW, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	defer d.Abort()
	if err = copyFile(d.File, s); err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	if err = d.Close(); err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	// now set mtime/ctime, mode etc.
	if err = updateMeta(dst, s.Name(), fi); err != nil {
		return fmt.Errorf("clonefile: %w", err)
	}

	return nil
}
