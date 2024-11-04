// dir_test.go -- clone dir tests
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
	"fmt"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/opencoff/go-fio/cmp"
	"github.com/puzpuzpuz/xsync/v3"
)

// clone empty dirs
func TestTreeCloneEmpty(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)
	src := path.Join(tmp, "empty", "lhs")
	dst := path.Join(tmp, "empty", "rhs")

	err := os.MkdirAll(src, 0700)
	assert(err == nil, "mkdir src: %s: %s", src, err)

	err = os.MkdirAll(dst, 0700)
	assert(err == nil, "mkdir dst: %s: %s", dst, err)

	err = Tree(dst, src)
	assert(err == nil, "clone: %s", err)

	// now run a cmp to ensure there are no differences
	err = treeEq(src, dst, t)
	assert(err == nil, "cmp: %s", err)
}

// clone dirs with a few entries on the lhs
func TestTreeCloneBasic(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)

	src := path.Join(tmp, "lhs")
	dst := path.Join(tmp, "rhs")

	err := os.MkdirAll(src, 0700)
	assert(err == nil, "mkdir src: %s: %s", src, err)

	err = os.MkdirAll(dst, 0700)
	assert(err == nil, "mkdir dst: %s: %s", dst, err)

	//err = mkfiles(src, []string{"a/b", "a/c"}, 3)
	err = mkfiles(src, []string{"a/b"}, 2)
	assert(err == nil, "mkfiles src: %s", err)

	//err = mkfiles(dst, []string{"a/b", "a/c"}, 3)
	err = mkfiles(dst, []string{"a/b"}, 2)
	assert(err == nil, "mkfiles src: %s", err)

	err = Tree(dst, src)
	assert(err == nil, "clone: %s", err)

	err = treeEq(src, dst, t)
	assert(err == nil, "cmp: %s", err)
}

// clone dirs with changes on both sides
func TestTreeCloneDiffs(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)

	src := path.Join(tmp, "lhs")
	dst := path.Join(tmp, "rhs")

	err := os.MkdirAll(src, 0700)
	assert(err == nil, "mkdir src: %s: %s", src, err)

	err = os.MkdirAll(dst, 0700)
	assert(err == nil, "mkdir dst: %s: %s", dst, err)

	err = mkfiles(src, []string{"a/b", "a/c", "a/d"}, 3)
	assert(err == nil, "mkfiles src: %s", err)

	err = mkfiles(dst, []string{"a/b", "a/c", "a/d"}, 2)
	assert(err == nil, "mkfiles src: %s", err)

	err = Tree(dst, src)
	assert(err == nil, "clone: %s", err)

	err = treeEq(src, dst, t)
	assert(err == nil, "cmp: %s", err)
}

func mkfiles(base string, paths []string, n int) error {
	for _, p := range paths {
		dn := path.Join(base, p)
		for i := 0; i < n; i++ {
			nm := fmt.Sprintf("f%03d", i)
			fn := path.Join(dn, nm)
			if err := mkfilex(fn); err != nil {
				return err
			}
		}
	}
	return nil
}

func treeEq(src, dst string, t *testing.T) error {
	d, err := cmp.DirTree(src, dst)
	if err != nil {
		return err
	}

	//t.Logf("%s\n", d)

	if d.Funny.Size() > 0 {
		return xerror("funny", d.Funny)
	}

	if d.LeftDirs.Size() > 0 {
		return xerror("left-dirs", d.LeftDirs)
	}
	if d.LeftFiles.Size() > 0 {
		return xerror("left-files", d.LeftFiles)
	}

	if d.RightDirs.Size() > 0 {
		return xerror("right-dirs", d.RightDirs)
	}
	if d.RightFiles.Size() > 0 {
		return xerror("right-files", d.RightFiles)
	}

	if d.Diff.Size() > 0 {
		return xerror("diff", d.Diff)
	}
	return nil
}

func xerror[K string, V any](pref string, m *xsync.MapOf[string, V]) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%s:\n", pref)
	m.Range(func(nm string, _ V) bool {
		fmt.Fprintf(&b, "\t%s\n", nm)
		return true
	})

	return fmt.Errorf("error - %s", b.String())
}
