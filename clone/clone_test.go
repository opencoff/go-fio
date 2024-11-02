// clone_test.go -- clone file tests
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
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
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"testing"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-mmap"
)

func TestCloneDir(t *testing.T) {
	assert := newAsserter(t)

	tmp := getTmpdir(t)
	nm := path.Join(tmp, "testdir")
	err := os.MkdirAll(nm, 0700)
	assert(err == nil, "mkdir: %s", err)

	x := fio.Xattr{
		"user.dir.name": nm,
	}

	err = fio.SetXattr(nm, x)
	assert(err == nil, "setxattr: %s", err)

	dst := path.Join(tmp, "newdir")
	err = File(dst, nm)
	assert(err == nil, "clonedir: %s", err)

	// now fetch all the attrs of newdir and make sure they're identical
	// to the src
	err = mdEqual(dst, nm)
	assert(err == nil, "clonedir: %s", err)
}

func TestCloneRegFile(t *testing.T) {
	assert := newAsserter(t)

	tmp := getTmpdir(t)
	nm := path.Join(tmp, "testfile")
	err := mkfilex(nm)
	assert(err == nil, "test file %s: %s", nm, err)

	x := fio.Xattr{
		"user.file.name": nm,
	}

	err = fio.SetXattr(nm, x)
	assert(err == nil, "setxattr: %s", err)

	dst := path.Join(tmp, "newfile")
	err = File(dst, nm)
	assert(err == nil, "clonereg: %s", err)

	// now fetch all the attrs of newdir and make sure they're identical
	// to the src
	err = mdEqual(dst, nm)
	assert(err == nil, "clonereg: %s", err)
}

func TestCloneSymlink(t *testing.T) {
	assert := newAsserter(t)

	tmp := getTmpdir(t)
	nm := path.Join(tmp, "testfile")
	err := mkfilex(nm)
	assert(err == nil, "test file %s: %s", nm, err)

	newnm := path.Join(tmp, "symlink")
	linknm := "./testfile"
	err = os.Symlink(linknm, newnm)
	assert(err == nil, "symlink: %s", err)

	nm2 := path.Join(tmp, "new-link")
	err = File(nm2, newnm)
	assert(err == nil, "clonelink: %s", err)

	// verify that the link contents are readable
	vlink, err := os.Readlink(nm2)
	assert(err == nil, "readlink: %s", err)
	assert(vlink == linknm, "link mismatch: exp %s, saw %s", linknm, vlink)

	err = mdEqual(nm2, newnm)
	assert(err == nil, "clonelink: %s", err)
}

func mdEqual(newf, oldf string) error {
	a, err := fio.Lstat(oldf)
	if err != nil {
		return err
	}
	b, err := fio.Lstat(newf)
	if err != nil {
		return err
	}

	if (a.Mod & ^fs.ModePerm) != (b.Mod & ^fs.ModePerm) {
		return fmt.Errorf("mode: exp %#x, saw %#x", a.Mod, b.Mod)
	}

	if a.Nlink != b.Nlink {
		return fmt.Errorf("nlink: exp %d, saw %d", a.Nlink, b.Nlink)
	}
	if a.Uid != b.Uid {
		return fmt.Errorf("uid: exp %d, saw %d", a.Uid, b.Uid)
	}
	if a.Gid != b.Gid {
		return fmt.Errorf("gid: exp %d, saw %d", a.Gid, b.Gid)
	}
	if a.Siz != b.Siz {
		return fmt.Errorf("size: exp %d, saw %d", a.Siz, b.Siz)
	}
	if a.Dev != b.Dev {
		return fmt.Errorf("dev: exp %d, saw %d", a.Dev, b.Dev)
	}
	if a.Rdev != b.Rdev {
		return fmt.Errorf("rdev: exp %d, saw %d", a.Rdev, b.Rdev)
	}

	if a.Mode().Type() != fs.ModeSymlink {
		if !a.Mtim.Equal(b.Mtim) {
			return fmt.Errorf("mtime:\n\texp %s\n\tsaw %s", a.Mtim, b.Mtim)
		}
	}

	done := make(map[string]bool)
	for k, v := range a.Xattr {
		v2, ok := b.Xattr[k]
		if !ok {
			return fmt.Errorf("xattr: missing %s", k)
		}
		if v2 != v {
			return fmt.Errorf("xattr: %s: exp %s, saw %s", k, v, v2)
		}
		done[k] = true
	}

	for k := range b.Xattr {
		_, ok := done[k]
		if !ok {
			return fmt.Errorf("xattr: unknown key %s", k)
		}
	}

	if !a.IsRegular() {
		return nil
	}

	// content equality for regular files
	rfd, err := os.Open(oldf)
	if err != nil {
		return err
	}
	defer rfd.Close()

	wfd, err := os.Open(newf)
	if err != nil {
		return err
	}
	defer wfd.Close()

	rm := mmap.New(rfd)
	wm := mmap.New(wfd)

	rmm, err := rm.Map(-1, 0, mmap.PROT_READ, mmap.F_READAHEAD)
	if err != nil {
		return err
	}
	defer rmm.Unmap()

	wmm, err := wm.Map(-1, 0, mmap.PROT_READ, mmap.F_READAHEAD)
	if err != nil {
		return err
	}
	defer wmm.Unmap()

	if !bytes.Equal(rmm.Bytes(), wmm.Bytes()) {
		return fmt.Errorf("content: mismatch")
	}

	return nil
}
