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
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/walk"
)


type Opt func(o *opts)

type opt struct {
	walk.Options

	deepEq func(lhs, rhs *fio.Info) bool
}

func WithWalkOptions(wo *walk.Options) Opt {
	return func(o *opts) {
		o.Options = *wo
	}
}


func WithDeepCompare(same func(lhs, rhs *fio.Info) bool) Opt {
	return func(o *opts) {
		o.deepEq = same
	}
}


type Difference struct {
	// Each of these maps uses a relative path as the key - it
	// is relative to the argument passed to NewTree().
	Left  map[string]*fio.Info
	Right map[string]*fio.Info

	// entries that are only on the left
	LeftOnly  []string

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

type Cmp struct {
	opt
}

// given to workers to figure out actual differences
type work struct {
	lhs *fio.Info
	rhs *fio.Info
}

func New(lhs, rhs string, o ...Opt) (*Cmp, error) {
}


func (e *Cmp) Diff() (*Difference, error) {
	left, err := lhs.gather()
	if err != nil {
		return nil, err
	}

	right, err := rhs.gather()
	if err != nil {
		return nil, err
	}

	// now process each side and build up differences
	for nm, li := range left {
		ri, ok := right[nm]
		if !ok {
			lo = append(lo, nm)
			continue
		}

		if (li.Mod & ^fs.ModePerm) != (ri.Mod & ^fs.ModePerm) {
			// funny business
			funny = append(funny, nm)
		} else {
			// submit work for workers to handle
		}
	}

	var wg sync.WaitGroup
}




func (t *Tree) gather() (map[string]*fio.Info, error) {
	tree := make(map[string]*fio.Info)

	// setup a walk instance and gather entries

	och, ech := walk.Walk([]string{t.nm}, &t.opt.Options)

	var errs []error
	var wg sync.WaitGroup

	wg.Add(1)
	go func(wg *sync.WaitGroup, ch chan error) {
		for e := range ch {
			errs = append(errs, e)
		}
		wg.Done()
	}(&wg, ech)

	n := len(t.nm)
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

