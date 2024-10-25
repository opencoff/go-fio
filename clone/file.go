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

package clone

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/opencoff/go-fio"
)

// CloneMetadata clones all the metadata from src to dst: the metadata
// is atime, mtime, uid, gid, mode/perm, xattr
func Metadata(dst, src string) error {
	fi, err := fio.Lstat(src)
	if err != nil {
		return &Error{"stat-src", src, dst, err}
	}

	return updateMeta(dst, fi)
}

// UpdateMetadata writes new metadata of 'dst' from 'fi'
// The metadata that will be updated includes atime, mtime, uid/gid,
// mode/perm, xattr
func UpdateMetadata(dst string, fi *fio.Info) error {
	return updateMeta(dst, fi)
}

// File clones src to dst - including all clonable file attributes
// and xattr. File will use the best available CoW facilities provided
// by the OS and Filesystem. It will fall back to using copy via mmap(2) on
// systems that don't have CoW semantics.
func File(dst, src string) error {
	fi, err := fio.Lstat(src)
	if err != nil {
		return &Error{"stat-src", src, dst, err}
	}

	s, err := os.Open(src)
	if err != nil {
		return &Error{"open-src", src, dst, err}
	}

	defer s.Close()

	mode := fi.Mode()
	if mode.IsRegular() {
		if err = copyRegular(dst, s, fi); err != nil {
			return err
		}
		goto done
	}

	switch mode.Type() {
	case fs.ModeDir:
		if err = os.MkdirAll(dst, mode&fs.ModePerm|0100); err != nil {
			return &Error{"mkdir", src, dst, err}
		}

	case fs.ModeSymlink:
		if err = clonelink(dst, src, fi); err != nil {
			return &Error{"clonelink", src, dst, err}
		}

	case fs.ModeDevice:
		if err = mknod(dst, fi); err != nil {
			return &Error{"mknod", src, dst, err}
		}

	//case ModeSocket: XXX Add named socket support

	default:
		err = fmt.Errorf("unsupported type %#x", mode)
		return &Error{"file-type", src, dst, err}
	}

done:
	return updateMeta(dst, fi)
}

// copy a regular file to another regular file
func copyRegular(dst string, s *os.File, fi *fio.Info) error {
	// make the intermediate dirs of the dest
	dn := filepath.Dir(dst)
	if err := os.MkdirAll(dn, 0100|fs.ModePerm&fi.Mode()); err != nil {
		return &Error{"mkdir", s.Name(), dst, err}
	}

	// We create the file so that we can write to it; we'll update the perm bits
	// later on
	d, err := fio.NewSafeFile(dst, fio.OPT_COW|fio.OPT_OVERWRITE, os.O_CREATE|os.O_RDWR|os.O_EXCL, 0600)
	if err != nil {
		return &Error{"safefile", s.Name(), dst, err}
	}
	defer d.Abort()

	if err = fio.CopyFd(d.File, s); err != nil {
		return &Error{"copyfile", s.Name(), dst, err}
	}
	if err = d.Close(); err != nil {
		return &Error{"close", s.Name(), dst, err}
	}
	return nil
}

// a cloner clones a specific attribute
type cloner func(dst string, src *fio.Info) error

// all fs entries will have these attrs cloned.
// We stack mtime update to the end.
var mdUpdaters = []cloner{
	clonexattr,
	cloneugid,
	clonemode,
	clonetimes,
}

func clonexattr(dst string, fi *fio.Info) error {
	if err := fio.LreplaceXattr(dst, fi.Xattr); err != nil {
		return &Error{"replace-xattr", fi.Name(), dst, err}
	}
	return nil
}

func cloneugid(dst string, fi *fio.Info) error {
	if err := os.Lchown(dst, int(fi.Uid), int(fi.Gid)); err != nil {
		return &Error{"lchown", fi.Name(), dst, err}
	}
	return nil
}

func clonemode(dst string, fi *fio.Info) error {
	if err := os.Chmod(dst, fi.Mode()); err != nil {
		return &Error{"chmod", fi.Name(), dst, err}
	}
	return nil
}

func updateMeta(dst string, fi *fio.Info) error {
	for _, fp := range mdUpdaters {
		if err := fp(dst, fi); err != nil {
			return &Error{"md-update", fi.Name(), dst, err}
		}
	}
	return nil
}
