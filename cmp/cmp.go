// cmp.go -- concurrent directory differencing
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
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/walk"
)

type IgnoreFlag uint

const (
	IGN_UID IgnoreFlag = 1 << iota
	IGN_GID
	IGN_HARDLINK
	IGN_XATTR
)

type Opt func(o *opt)

type opt struct {
	ncpu   int
	deepEq func(lhs, rhs *fio.Info) bool
	ignore IgnoreFlag
}

func WithIgnore(fl IgnoreFlag) Opt {
	return func(o *opt) {
		o.ignore |= fl
	}
}

func WithDeepCompare(same func(lhs, rhs *fio.Info) bool) Opt {
	return func(o *opt) {
		o.deepEq = same
	}
}

type Difference struct {
	// Each of these maps uses a relative path as the key - it
	// is relative to the argument passed to NewTree().
	Left  map[string]*fio.Info
	Right map[string]*fio.Info

	// entries that are only on the left
	LeftOnly []string

	// entries that are only on the right
	RightOnly []string

	// Entries that are identical
	Same []string

	// entries that are different (size, perm, uid, gid, xattr)
	Diff []string

	// entries with same name on both sides but
	// are different (eg entry is a file on one side
	// but a directory in the other)
	Funny []string
}

type differ struct {
	opt

	lhs *Tree
	rhs *Tree

	fileEq func(lhs, rhs *fio.Info) bool

	same [][]string
	diff [][]string
}

func DirCmp(lhs, rhs *Tree, op ...Opt) (*Difference, error) {
	d := &differ{
		opt: opt{
			ncpu: runtime.NumCPU(),
		},
		lhs: lhs,
		rhs: rhs,
	}
	opts := &d.opt

	for _, o := range op {
		o(opts)
	}

	if d.ncpu <= 0 {
		d.ncpu = runtime.NumCPU()
	}

	// some form of sanity check?
	if d.ncpu > 4096 {
		panic(fmt.Sprintf("cmp: %d is too many CPUs?", d.ncpu))
	}

	// shard the results per go-routine; we'll combine them
	// later on
	d.same = make([][]string, d.ncpu)
	d.diff = make([][]string, d.ncpu)

	eqv := makeComparators(opts)
	d.fileEq = func(lhs, rhs *fio.Info) bool {
		for _, eq := range eqv {
			if !eq(lhs, rhs) {
				return false
			}
		}
		return true
	}

	left, err := d.lhs.gather()
	if err != nil {
		return nil, err
	}

	right, err := rhs.gather()
	if err != nil {
		return nil, err
	}

	// start workers to do per-file diff
	var wg sync.WaitGroup

	och := make(chan work, d.ncpu)

	wg.Add(d.ncpu)
	for i := 0; i < d.ncpu; i++ {
		go d.worker(i, och, &wg)
	}

	var lo, ro []string
	var funny, same, diff []string

	done := make(map[string]bool, len(left))

	for nm, li := range left {
		ri, ok := right[nm]
		if !ok {
			lo = append(lo, nm)
			continue
		}

		done[nm] = true
		if (li.Mod & ^fs.ModePerm) != (ri.Mod & ^fs.ModePerm) {
			// funny business
			funny = append(funny, nm)
		} else {
			// submit work for workers to handle
			och <- work{li, ri}
		}
	}

	for nm, ri := range right {
		li, ok := left[nm]
		if !ok {
			ro = append(ro, nm)
			continue
		}

		if _, ok := done[nm]; !ok {
			continue
		}

		if (ri.Mod & ^fs.ModePerm) != (li.Mod & ^fs.ModePerm) {
			// funny business
			funny = append(funny, nm)
		} else {
			// submit work for workers to handle
			// XXX should never happen
			och <- work{li, ri}
		}
	}

	// wait for workers to complete
	close(och)
	wg.Wait()

	// collect each of their results
	for i := 0; i < d.ncpu; i++ {
		same = append(same, d.same[i]...)
		diff = append(diff, d.diff[i]...)
	}

	result := &Difference{
		Left:      left,
		Right:     right,
		LeftOnly:  lo,
		RightOnly: ro,
		Same:      same,
		Diff:      diff,
		Funny:     funny,
	}

	return result, nil
}

// given to workers to figure out actual differences
type work struct {
	lhs *fio.Info
	rhs *fio.Info
}

// worker to compare each file and classify as same or different
// The workers _ONLY_ process files that are present on both sides
func (d *differ) worker(me int, och chan work, wg *sync.WaitGroup) {
	var same, diff []string

	for w := range och {
		if d.fileEq(w.lhs, w.rhs) {
			same = append(same, w.lhs.Name())
		} else {
			diff = append(diff, w.lhs.Name())
		}
	}

	d.same[me] = same
	d.diff[me] = diff
	wg.Done()
}

type fileqFunc func(a, b *fio.Info) bool

func makeComparators(opts *opt) []fileqFunc {
	ignore := func(fl IgnoreFlag) bool {
		if fl&opts.ignore > 0 {
			return true
		}
		return false
	}

	eqv := make([]fileqFunc, 0, 5)

	// We always have the most basic comparator: file size and mtime
	eqv = append(eqv, func(lhs, rhs *fio.Info) bool {
		if lhs.Size() != rhs.Size() {
			return false
		}

		if !lhs.Mtim.Equal(rhs.Mtim) {
			return false
		}
		return true
	})

	// build out the rest of optional comparators
	if !ignore(IGN_UID) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) bool {
			return lhs.Uid == rhs.Uid
		})
	}
	if !ignore(IGN_GID) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) bool {
			return lhs.Gid == rhs.Gid
		})
	}
	if !ignore(IGN_HARDLINK) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) bool {
			return lhs.Nlink == rhs.Nlink
		})
	}
	if !ignore(IGN_XATTR) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) bool {
			return lhs.Xattr.Equal(rhs.Xattr)
		})
	}

	// we want potentially expensive comparisons to be done last.
	if opts.deepEq != nil {
		eqv = append(eqv, opts.deepEq)
	}

	return eqv
}

type TreeOpt func(o *treeopt)
type treeopt struct {
	walk.Options
}

func WithWalkOptions(wo *walk.Options) TreeOpt {
	return func(o *treeopt) {
		o.Options = *wo
	}
}

type Tree struct {
	treeopt

	dir  string
	root *fio.Info
}

func NewTree(nm string, opts ...TreeOpt) (*Tree, error) {
	fi, err := fio.Lstat(nm)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("tree: %s is not a dir", nm)
	}

	o := &treeopt{}
	for _, fp := range opts {
		fp(o)
	}

	t := &Tree{
		treeopt: *o,
		dir:     nm,
		root:    fi,
	}

	return t, nil
}

func (t *Tree) gather() (map[string]*fio.Info, error) {
	tree := make(map[string]*fio.Info)

	// setup a walk instance and gather entries
	och, ech := walk.Walk([]string{t.dir}, &t.treeopt.Options)

	var errs []error
	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup, ch chan error) {
		for e := range ch {
			errs = append(errs, e)
		}
		wg.Done()
	}(&wg, ech)

	n := len(t.dir)
	for ii := range och {
		nm := ii.Name()
		if len(nm) > n {
			nm = nm[n+1:]
		}
		tree[nm] = ii
	}

	wg.Wait()
	if len(errs) > 0 {
		return tree, errors.Join(errs...)
	}
	return tree, nil
}
