// tree.go - clone a dir tree recursively
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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
)

type cloneopt struct {
	walk.Options

	// file-sys attributes to ignore for equality comparison
	// Used by cmp.DirCmp
	ignoreAttr cmp.IgnoreFlag
}

func defaultOpts() cloneopt {
	return cloneopt{
		Options: walk.Options{
			Concurrency:    runtime.NumCPU(),
			Type:           walk.ALL,
			OneFS:          false,
			FollowSymlinks: false,
			Excludes:       []string{".zfs"},
		},
	}
}

// Option captures the various options for cloning
// a directory tree.
type Option func(o *cloneopt)

// WithIgnoreAttr captures the attributes of fio.Info that must be
// ignored for comparing equality of two filesystem entries.
func WithIgnoreAttr(fl cmp.IgnoreFlag) Option {
	return func(o *cloneopt) {
		o.ignoreAttr = fl
	}
}

// WithWalkOptions uses 'wo' as the option for walk.Walk(); it
// describes a caller desired traversal of the file system with
// the requisite input and output filters
func WithWalkOptions(wo walk.Options) Option {
	return func(o *cloneopt) {
		o.Options = wo

		// make sure we receive all input
		if o.Type == 0 {
			o.Type = walk.ALL
		}

		if o.Concurrency <= 0 {
			o.Concurrency = runtime.NumCPU()
		}
	}
}

// Tree clones the entire file system tree 'src' into 'dst'.
// ie entries like src/a/b are cloned into dst/a/b. Returns
// Error or nil
func Tree(dst, src string, opts ...Option) error {
	opt := defaultOpts()
	for _, fp := range opts {
		fp(&opt)
	}

	tc, err := newTreeCloner(src, dst, &opt)
	if err != nil {
		return err
	}

	tc.workfp = tc.apply
	return tc.sync()
}

type treeCloner struct {
	cloneopt

	src, dst string

	diff *cmp.Difference

	workch chan op
	ech    chan error

	workfp func(o op) error
	out    io.Writer
}

// operation to apply and its data
// for opCp: a, b are both valid
// for opRm: only a is valid
type op struct {
	typ  opType
	a, b string

	lhs, rhs *fio.Info
}

type opType int

const (
	opCp opType = 1 + iota // copy a to b
	opRm                   // rm a
)

// clone src to dst; we know both are dirs
func newTreeCloner(src, dst string, opt *cloneopt) (*treeCloner, error) {
	if err := validate(src, dst); err != nil {
		return nil, err
	}

	ltree, err := cmp.NewTree(src, cmp.WithWalkOptions(opt.Options))
	if err != nil {
		return nil, err
	}

	rtree, err := cmp.NewTree(dst, cmp.WithWalkOptions(opt.Options))
	if err != nil {
		return nil, err
	}

	diff, err := cmp.DirCmp(ltree, rtree, cmp.WithIgnore(cmp.IGN_HARDLINK))
	if err != nil {
		return nil, err
	}

	tc := &treeCloner{
		cloneopt: *opt,
		src:      src,
		dst:      dst,
		diff:     diff,
		workch:   make(chan op, opt.Concurrency),
		ech:      make(chan error, 1),
	}

	return tc, nil
}

func validate(src, dst string) error {
	// first make the dir
	di, err := fio.Lstat(dst)
	if err == nil {
		if !di.IsDir() {
			return &Error{"lstat", src, dst, fmt.Errorf("destination is not a directory")}
		}
	}

	if err != nil {
		if os.IsNotExist(err) {
			// make the directory if needed
			err = File(dst, src)
		}

		if err != nil {
			return &Error{"lstat", src, dst, err}
		}
	}
	return nil
}

func (tc *treeCloner) apply(o op) error {
	switch o.typ {
	case opCp:
		if err := File(o.b, o.a); err != nil {
			return &Error{"clone-entry", o.a, o.b, err}
		}
	case opRm:
		if err := os.RemoveAll(o.a); err != nil {
			return &Error{"open-src", o.a, o.b, err}
		}
	default:
		return &Error{"unknown-op", o.a, o.b, fmt.Errorf("unknown op %d", o.typ)}
	}
	return nil
}

func (tc *treeCloner) worker(wg *sync.WaitGroup) {
	for o := range tc.workch {
		err := tc.workfp(o)
		if err != nil {
			tc.ech <- err
		}
	}
	wg.Done()
}

func (tc *treeCloner) sync() error {
	var errs []error
	var ewg, wg sync.WaitGroup

	// harvest errors
	ewg.Add(1)
	go func() {
		for e := range tc.ech {
			errs = append(errs, e)
		}
		ewg.Done()
	}()

	// start workers
	wg.Add(tc.Concurrency)
	for i := 0; i < tc.Concurrency; i++ {
		go tc.worker(&wg)
	}

	// And, queue up work for the workers
	var submitDone sync.WaitGroup

	diff := tc.diff
	submitDone.Add(3)
	go func(wg *sync.WaitGroup) {
		for _, nm := range diff.Diff {
			s, ok := diff.Left[nm]
			if !ok {
				err := fmt.Errorf("%s: can't find in left map", nm)
				tc.ech <- &Error{"clonetree", tc.src, tc.dst, err}
				continue
			}

			d, ok := diff.Right[nm]
			if !ok {
				err := fmt.Errorf("%s: can't find in right map", nm)
				tc.ech <- &Error{"clonetree", tc.src, tc.dst, err}
				continue
			}

			o := op{
				typ: opCp,
				a:   s.Name(),
				b:   d.Name(),
				lhs: s,
				rhs: d,
			}
			tc.workch <- o
		}
		wg.Done()
	}(&submitDone)

	go func(wg *sync.WaitGroup) {
		for _, nm := range diff.LeftOnly {
			s, ok := diff.Left[nm]
			if !ok {
				err := fmt.Errorf("%s: can't find in left map", nm)
				tc.ech <- &Error{"clonetree", tc.src, tc.dst, err}
				continue
			}

			o := op{
				typ: opCp,
				a:   s.Name(),
				b:   path.Join(tc.dst, nm),
				lhs: s,
			}
			tc.workch <- o
		}
		wg.Done()
	}(&submitDone)

	go func(wg *sync.WaitGroup) {
		for _, nm := range diff.RightOnly {
			d, ok := diff.Right[nm]
			if !ok {
				err := fmt.Errorf("diff: can't find %s in destination", nm)
				tc.ech <- &Error{"clonetree", tc.src, tc.dst, err}
				continue
			}

			o := op{
				typ: opRm,
				a:   d.Name(),
				lhs: d,
			}
			tc.workch <- o
		}
		wg.Done()
	}(&submitDone)

	// when we're done submitting all the work, close the worker input chan
	go func() {
		submitDone.Wait()
		close(tc.workch)
	}()

	// now wait for workers to complete
	wg.Wait()

	// wait for error harvestor to be complete
	close(tc.ech)
	ewg.Wait()

	if len(errs) > 0 {
		return &Error{"clonetree", tc.src, tc.dst, errors.Join(errs...)}
	}
	return nil
}
