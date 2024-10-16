// walk_test.go -- test harness for walk.go

package fio

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

type test struct {
	dir string
	typ Type
}

var tests = []test{
	//{"$HOME/.config", FILE | SYMLINK},
}

var linuxTests = []test{
	{"/dev", ALL},
	{"/dev", DEVICE},
	{"/dev", SPECIAL},
	{"/dev", DIR | SYMLINK},
	{"/lib", FILE | SYMLINK},
	{"/lib", SYMLINK},
	{"/bin", FILE},
}

var macOSTests = []test{
	{"/etc", ALL},
	{"/bin", FILE},
	{"$HOME/Library", ALL},
	{"$HOME/Library", FILE | SYMLINK},
}

func (tx *test) String() string {
	return fmt.Sprintf("%s:%s", tx.dir, tx.typ.String())
}

func TestWalk(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		tests = append(tests, linuxTests...)

	case "darwin":
		tests = append(tests, macOSTests...)
	}

	for i := range tests {
		tx := &tests[i]

		t.Run(tx.String(), func(t *testing.T) {
			compareWalks(tx, t)
		})
	}
}

func TestWalkSimple(t *testing.T) {
	assert := newAsserter(t)
	tmpdir := t.TempDir()
	err := mkTestDir(tmpdir)
	assert(err == nil, "mktmp: %s", err)

	tests := []test{
		{tmpdir, FILE},
		{tmpdir, DIR},
		{tmpdir, SYMLINK},
		{tmpdir, FILE | SYMLINK},
	}

	for i := range tests {
		tx := &tests[i]
		compareWalks(tx, t)
	}

	//os.RemoveAll(tmpdir)
}

func compareWalks(tx *test, t *testing.T) {
	assert := newAsserter(t)

	var wg sync.WaitGroup
	var r1, r2 map[string]fs.FileInfo
	var e1, e2 error

	wg.Add(2)
	go func(tx *test) {
		r2, e2 = newWalk(tx)
		wg.Done()
	}(tx)

	go func(tx *test) {
		r1, e1 = oldWalk(tx)
		wg.Done()
	}(tx)

	wg.Wait()
	assert(e2 == nil, "%s: Errors new-walk:\n%s\n", tx, e2)
	assert(e1 == nil, "%s: Errors old-walk:\n%s\n", tx, e1)

	for k := range r1 {
		_, ok := r2[k]
		assert(ok, "%s: can't find %s in new walk", tx, k)
		delete(r2, k)
	}

	// now we know that everything the stdlib.Walk found is also present
	// in our concurrent-walker.

	if len(r2) > 0 {
		var rem []string

		for k := range r2 {
			rem = append(rem, k)
		}
		t.Fatalf("new walk has extra entries:\n%s\n",
			strings.Join(rem, "\n"))
	}
}

// make a test dir with known entries
func mkTestDir(tmpdir string) error {
	var err error

	if err = mkfile(tmpdir, "a"); err != nil {
		return err
	}

	if err = mkfile(tmpdir, "b/c/d"); err != nil {
		return err
	}

	if err = mkfile(tmpdir, "b/c/e"); err != nil {
		return err
	}

	if err = mksym(tmpdir, "b/c/e", "b/symlink"); err != nil {
		return err
	}

	return nil
}

func mkfile(tmpdir, p string) error {
	fn := filepath.Join(tmpdir, p)
	return mkfilex(fn)
}

func mkfilex(fn string) error {
	bn := filepath.Dir(fn)
	if err := os.MkdirAll(bn, 0700); err != nil {
		return fmt.Errorf("mkdir: %s: %w", bn, err)
	}

	fd, err := os.OpenFile(fn, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("creat: %s: %w", fn, err)
	}

	fd.Write([]byte("hello"))
	fd.Sync()
	return fd.Close()
}

// make a symlink named 'src' to point to 'targ'
func mksym(tmpdir string, src, targ string) error {
	s := filepath.Join(tmpdir, src)
	d := filepath.Join(tmpdir, targ)
	if err := os.Symlink(s, d); err != nil {
		return err
	}
	return nil
}

func newWalk(tx *test) (map[string]fs.FileInfo, error) {
	nm := os.ExpandEnv(tx.dir)
	names := [...]string{nm}
	opt := &Options{
		FollowSymlinks: false,
		OneFS:          false,
		Type:           tx.typ,
	}

	res := make(map[string]fs.FileInfo)
	och, ech := Walk(names[:], opt)

	var wg sync.WaitGroup

	wg.Add(1)
	var errs []error
	go func() {
		for e := range ech {
			errs = append(errs, e)
		}
		wg.Done()
	}()

	for o := range och {
		res[o.Name()] = o
	}

	wg.Wait()
	if len(errs) > 0 {
		return res, errors.Join(errs...)
	}
	return res, nil
}

func oldWalk(tx *test) (map[string]fs.FileInfo, error) {
	var m os.FileMode

	ty := tx.typ
	for k, v := range typMap {
		if (k & ty) > 0 {
			m |= v
		}
	}

	predicate := func(mode fs.FileMode) bool {
		if (m&mode) > 0 || ((ty&FILE) > 0 && mode.IsRegular()) {
			return true
		}
		return false
	}

	res := make(map[string]fs.FileInfo)
	var errs []error

	nm := os.ExpandEnv(tx.dir)
	err := filepath.WalkDir(nm, func(p string, di fs.DirEntry, e error) error {
		if e != nil {
			errs = append(errs, e)
			return nil
		}

		if !predicate(di.Type()) {
			return nil
		}

		// we're interested in this entry
		fi, err := di.Info()
		if err != nil {
			errs = append(errs, err)
		} else {
			res[p] = fi
		}
		return nil
	})

	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return res, errors.Join(errs...)
	}
	return res, nil
}
