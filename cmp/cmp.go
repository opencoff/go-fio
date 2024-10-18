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

// IgnoreFlag captures the attributes we want to ignore while comparing
// two fio.Info instances representing two filesystem entries on the
// two trees being compared.
type IgnoreFlag uint

const (
	IGN_UID      IgnoreFlag = 1 << iota // ignore uid
	IGN_GID                             // ignore gid
	IGN_HARDLINK                        // ignore hardlink count
	IGN_XATTR                           // ignore xattr
)

// Opt is an option operator for DirCmp.
type Opt func(o *opt)

type opt struct {
	ncpu   int
	deepEq func(lhs, rhs *fio.Info) bool
	ignore IgnoreFlag
}

// WithIgnore captures the attributes of fio.Info that must be
// ignored for comparing equality of two filesystem entries.
func WithIgnore(fl IgnoreFlag) Opt {
	return func(o *opt) {
		o.ignore |= fl
	}
}

// WithDeepCompare provides a caller supplied comparison function
// that will be invoked if all other comparable attributes are
// identical.
func WithDeepCompare(same func(lhs, rhs *fio.Info) bool) Opt {
	return func(o *opt) {
		o.deepEq = same
	}
}

// Difference captures the results of comparing two trees
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

type fileqFunc func(a, b *fio.Info) bool

// diff engine state for internal use
type differ struct {
	opt

	lhs *Tree
	rhs *Tree

	fileEq fileqFunc

	same [][]string
	diff [][]string
}

// DirCmp compares two directory trees represented by "lhs" and "rhs".
// For regular files, it compares file size and mtime to determine change.
// For all entries, it compares every comparable attribute of fio.Info - unless
// explicitly ignored (by using the option WithIgnore()).
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

	// shard the results per go-routine; we'll combine them
	// later on
	d.same = make([][]string, d.ncpu)
	d.diff = make([][]string, d.ncpu)

	d.fileEq = makeComparators(opts)

	left, err := d.lhs.gather()
	if err != nil {
		return nil, err
	}

	right, err := rhs.gather()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var lo, ro, funny []string

	// start workers to do per-file diff
	och := make(chan work, d.ncpu)
	wg.Add(d.ncpu)
	for i := 0; i < d.ncpu; i++ {
		go d.worker(i, och, &wg)
	}

	done := make(map[string]bool, len(left))

	// first iterate over entries on the left
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

	// now see what remains on the right
	for nm := range right {
		_, ok := left[nm]
		if !ok {
			ro = append(ro, nm)
			continue
		}

		if _, ok := done[nm]; ok {
			continue
		}

		// NB: This case should never happen: one of the following
		//     must be true:
		//       a) file is NOT in LHS => it's only in the RHS
		//       b) file is already processed
		//
		panic(fmt.Sprintf("dircmp: rhs %s: WTF\n", nm))

		/*
			if (ri.Mod & ^fs.ModePerm) != (li.Mod & ^fs.ModePerm) {
				// funny business
				funny = append(funny, nm)
			} else {
				// submit work for workers to handle
				och <- work{li, ri}
			}
		*/
	}

	// wait for workers to complete
	close(och)
	wg.Wait()

	// collect each of the sharded results
	var same, diff []string
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
		// we know these are both of the same "type"
		lhs := w.lhs
		rhs := w.rhs
		nm := lhs.Name()

		if lhs.IsRegular() {
			// file size is only meaningful for regular files. So get that
			// out of the way
			if lhs.Size() != rhs.Size() {
				diff = append(diff, nm)
				continue
			}
		}

		// for all entries, compare the remaining attributes
		if d.fileEq(lhs, rhs) {
			same = append(same, nm)
		} else {
			diff = append(diff, nm)
		}
	}

	d.same[me] = same
	d.diff[me] = diff
	wg.Done()
}

// return a comparator function that is optimized for the attributes we are
// comparing
func makeComparators(opts *opt) fileqFunc {
	ignore := func(fl IgnoreFlag) bool {
		if fl&opts.ignore > 0 {
			return true
		}
		return false
	}

	eqv := make([]fileqFunc, 0, 6)

	// We always have the most basic comparator: mtime
	eqv = append(eqv, func(lhs, rhs *fio.Info) bool {
		return lhs.Mtim.Equal(rhs.Mtim)
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

	// the final function will call each of these smol comparators
	// one after the other.
	return func(a, b *fio.Info) bool {
		for _, fp := range eqv {
			if !fp(a, b) {
				return false
			}
		}
		return true
	}
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
