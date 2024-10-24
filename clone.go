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

// CloneMetadata clones all the metadata from src to dst: the metadata
// is atime, mtime, uid, gid, mode/perm, xattr
func CloneMetadata(dst, src string) error {
	fi, err := Lstat(src)
	if err == nil {
		err = updateMeta(dst, fi)
	}

	if err != nil {
		return fmt.Errorf("clonemeta: %w", err)
	}
	return nil
}

// UpdateMetadata writes new metadata of 'dst' from 'fi'
// The metadata that will be updated includes atime, mtime, uid/gid,
// mode/perm, xattr
func UpdateMetadata(dst string, fi *Info) error {
	if err := updateMeta(dst, fi); err != nil {
		return fmt.Errorf("updatemeta: %w", err)
	}
	return nil
}

// CloneFile copies src to dst - including all copyable file attributes
// and xattr. CloneFile will use the best available CoW facilities provided
// by the OS and Filesystem. It will fall back to using copy via mmap(2) on
// systems that don't have CoW semantics.
func CloneFile(dst, src string) error {
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
		err = copyRegular(dst, s, fi)
		goto done
	}

	switch mode.Type() {
	case fs.ModeDir:
		err = os.MkdirAll(dst, mode&fs.ModePerm|0100)

	case fs.ModeSymlink:
		err = clonelink(dst, src, fi)

	case fs.ModeDevice, fs.ModeNamedPipe:
		err = mknod(dst, fi)

	//case ModeSocket: XXX Add named socket support

	default:
		err = fmt.Errorf("clonefile: %s: unsupported type %#x", src, mode)
	}

done:
	if err == nil {
		// update metadata; caller is responsible for deep clone of
		// a directory.
		err = updateMeta(dst, fi)
	}

	// everyone must have their attrs cloned
	if err != nil {
		return fmt.Errorf("clonefile: %s from %s: %w", dst, src, err)
	}
	return nil
}

// copy a regular file to another regular file
func copyRegular(dst string, s *os.File, fi *Info) error {
	// make the intermediate dirs of the dest
	dn := filepath.Dir(dst)
	if err := os.MkdirAll(dn, 0100|fs.ModePerm&fi.Mode()); err != nil {
		return err
	}

	// We create the file so that we can write to it; we'll update the perm bits
	// later on
	d, err := NewSafeFile(dst, OPT_COW|OPT_OVERWRITE, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer d.Abort()

	di, err := Lstat(d.Name())
	if err != nil {
		return err
	}

	// if src and dest are on same fs, copy using the best OS primitive
	if di.IsSameFS(fi) {
		err = copyFile(d.File, s)
	} else {
		err = copyViaMmap(d.File, s)
	}

	if err == nil {
		err = d.Close()
	}
	return err
}

// a cloner clones a specific attribute
type cloner func(dst string, src *Info) error

// all fs entries will have these attrs cloned.
// We stack mtime update to the end.
var mdUpdaters = []cloner{
	clonexattr,
	cloneugid,
	clonemode,
	clonetimes,
}

func clonexattr(dst string, fi *Info) error {
	return LreplaceXattr(dst, fi.Xattr)
}

func cloneugid(dst string, fi *Info) error {
	return os.Lchown(dst, int(fi.Uid), int(fi.Gid))
}

func clonemode(dst string, fi *Info) error {
	return os.Chmod(dst, fi.Mode())
}

func updateMeta(dst string, fi *Info) error {
	for _, fp := range mdUpdaters {
		if err := fp(dst, fi); err != nil {
			return err
		}
	}
	return nil
}
