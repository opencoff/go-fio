// cmp_test.go -- test harness for dircmp
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

package fio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencoff/go-fio/walk"
)

func tmpDir(_ *testing.T, nm string) (string, error) {
	base := "/tmp/dircmp"
	dir := filepath.Join(base, nm)
	lhs := filepath.Join(dir, "lhs")
	rhs := filepath.Join(dir, "rhs")

	if err := os.MkdirAll(lhs, 0700); err != nil {
		return dir, fmt.Errorf("tmpdir: %s: %w", lhs, err)
	}
	if err := os.MkdirAll(rhs, 0700); err != nil {
		return dir, fmt.Errorf("tmpdir: %s: %w", rhs, err)
	}

	return dir, nil
}

func TestEmptyDir(t *testing.T) {
	assert := newAsserter(t)

	tdir, err := tmpDir(t, "empty")
	assert(err == nil, "%s", err)

	lhs := filepath.Join(tdir, "lhs")
	rhs := filepath.Join(tdir, "rhs")

	wo := &walk.Options{
		Concurrency: 4,
		Type: walk.ALL,
	}

	lt, err := NewTree(lhs, WithWalkOptions(wo))
	assert(err == nil, "%s", err)

	rt, err := NewTree(rhs, WithWalkOptions(wo))
	assert(err == nil, "%s", err)

	d, err := DirCmp(lt, rt)
	assert(err == nil, "%s", err)
	assert(d != nil, "diff is nil")

	fmt.Printf("diff: %s\n", d)

	// everything should be empty
	assert(len(d.LeftOnly) == 0, "leftonly %d", len(d.LeftOnly))
	assert(len(d.RightOnly) == 0, "rightonly %d", len(d.RightOnly))
	assert(len(d.Same) == 0, "rightonly %d", len(d.Same))
	assert(len(d.Diff) == 0, "rightonly %d", len(d.Diff))
	assert(len(d.Funny) == 0, "rightonly %d", len(d.Funny))

	os.RemoveAll(tdir)
}


func TestEmptyRhs(t *testing.T) {
	assert := newAsserter(t)

	tdir, err := tmpDir(t, "empty_rhs")
	assert(err == nil, "%s", err)

	lhs := filepath.Join(tdir, "lhs")
	rhs := filepath.Join(tdir, "rhs")

	wo := &walk.Options{
		Concurrency: 4,
		Type: walk.FILE,
	}

	lt, err := NewTree(lhs, WithWalkOptions(wo))
	assert(err == nil, "%s", err)

	rt, err := NewTree(rhs, WithWalkOptions(wo))
	assert(err == nil, "%s", err)

	// make the files needed on lhs
	files := "a/b/0 a/b/1 a/b/3 a/b/c/0 a/b/c/1"
	root := rootdir(lhs)
	fv := strings.Split(files, " ")
	for i := range fv {
		nm := fv[i]
		err := root.mkfile(nm)
		assert(err == nil, "%s", err)
		fmt.Printf("mkfile %s\n", nm)
	}

	d, err := DirCmp(lt, rt)
	assert(err == nil, "%s", err)
	assert(d != nil, "diff is nil")

	fmt.Printf("diff: %s\n", d)

	assert(len(d.LeftOnly) == len(fv), "leftonly: exp %d, saw %d", len(fv), len(d.LeftOnly))

	// rest should be empty
	assert(len(d.RightOnly) == 0, "rightonly %d", len(d.RightOnly))
	assert(len(d.Same) == 0, "rightonly %d", len(d.Same))
	assert(len(d.Diff) == 0, "rightonly %d", len(d.Diff))
	assert(len(d.Funny) == 0, "rightonly %d", len(d.Funny))

	os.RemoveAll(tdir)
}
