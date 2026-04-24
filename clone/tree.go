// tree.go - clone a dir-tree
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
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
	"github.com/puzpuzpuz/xsync/v3"
	"golang.org/x/sync/errgroup"
)

type Option func(o *treeopt)

// Observer is invoked when the tree cloner makes progress.
// The Difference method is called just before starting the
// I/O operation. For every entry that is processed, Tree()
// invokes the Copy or Delete methods. The final metadata
// fixup step is tracked by the MetadataUpdate method.
type Observer interface {
	cmp.Observer

	Difference(d *cmp.Difference)

	// mkdir dst
	Mkdir(dst string)

	// copy file src -> dst
	Copy(dst, src string)

	// delete file
	Delete(nm string)

	// create a hardlink src -> dst
	Link(dst, src string)

	MetadataUpdate(dst, src string)
}

// WithIgnoreAttr captures the attributes of fio.Info that must be
// ignored for comparing equality of two filesystem entries.
func WithIgnoreAttr(fl cmp.IgnoreFlag) Option {
	return func(o *treeopt) {
		o.fl = fl
	}
}

// WithWalkOptions uses 'wo' as the option for walk.Walk(); it
// describes a caller desired traversal of the file system with
// the requisite input and output filters
func WithWalkOptions(wo walk.Options) Option {
	return func(o *treeopt) {
		o.Options = wo
	}
}

// WithObserver uses 'ob' to report activities as the tree
// cloner makes progress
func WithObserver(ob Observer) Option {
	return func(o *treeopt) {
		o.o = ob
	}
}

// WithIgnoreMissing ensures that the cloner skips over
// files that disappear between the initial directory scan
// and concurrent differencing/copying.
func WithIgnoreMissing(ign bool) Option {
	return func(o *treeopt) {
		o.ignoreMissing = ign
	}
}

type treeopt struct {
	walk.Options

	// to report progress
	o Observer

	// skip files that disappeared
	ignoreMissing bool

	// file attrs to ignore while computing
	// file equality.
	fl cmp.IgnoreFlag
}

func defaultOptions() treeopt {
	opt := treeopt{
		Options: walk.Options{
			Concurrency: runtime.NumCPU(),
			Type:        walk.ALL,
		},
		o: NopObserver(),
	}
	return opt
}

// Tree clones the directory tree 'src' to 'dst' with options 'opt'.
// For example, an entry src/a will be cloned to dst/b. If dst
// exists, it must be a directory.
//
// Cancelling ctx aborts the traversal and any in-flight copies promptly.
func Tree(ctx context.Context, dst, src string, opt ...Option) error {
	si, err := fio.Lstat(src)
	if err != nil {
		return &Error{"lstat-src", src, dst, err}
	}
	if !si.IsDir() {
		return &Error{"clone", src, dst, fmt.Errorf("src is not a dir")}
	}

	di, err := fio.Lstat(dst)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return &Error{"lstat-dst", src, dst, err}
		}

		// first make the dest dir
		if err = File(dst, src); err != nil {
			return err
		}
	} else {
		if !di.IsDir() {
			return &Error{"clone", src, dst, fmt.Errorf("dst is not a dir")}
		}
	}

	option := defaultOptions()
	for _, fp := range opt {
		fp(&option)
	}

	diff, err := cmp.FsTree(ctx, src, dst, cmp.WithIgnoreAttr(option.fl),
		cmp.WithObserver(option.o),
		cmp.WithWalkOptions(option.Options))
	if err != nil {
		return &Error{"tree-diff", src, dst, err}
	}

	if diff.Funny.Size() > 0 {
		err := newFunnyError(diff.Funny)
		return &Error{"clone", src, dst, err}
	}

	n := newCloner(diff, &option)

	if err = n.clone(ctx); err != nil {
		return err
	}

	return nil
}

type dircloner struct {
	treeopt

	*cmp.Difference

	h *hardlinker

	// set of dst dirs modified during the clone. Populated concurrently
	// by worker goroutines; later walked sequentially by fixup.
	dirs *xsync.MapOf[string, bool]
}

func newCloner(d *cmp.Difference, opt *treeopt) *dircloner {
	cc := &dircloner{
		treeopt:    *opt,
		Difference: d,
		h:          newHardlinker(),
		dirs:       xsync.NewMapOf[string, bool](),
	}

	cc.o.Difference(d)

	return cc
}

func (cc *dircloner) xcopy(dst, src string) error {
	if err := File(dst, src); err != nil {
		if cc.ignoreMissing && errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func (cc *dircloner) clone(ctx context.Context) error {
	conc := cc.Concurrency
	if conc <= 0 {
		conc = runtime.NumCPU()
	}

	// Pass 1: create all new dirs before files. Errgroup fail-fast with
	// ctx cancel; the first mkdir failure aborts the rest.
	dirs := dirlist(cc.LeftDirs)
	eg, egctx := errgroup.WithContext(ctx)
	eg.SetLimit(conc)
	for _, nm := range dirs {
		nm := nm
		src := filepath.Join(cc.Src, nm)
		dst := filepath.Join(cc.Dst, nm)

		cc.dirs.Store(dst, true)
		cc.o.Mkdir(dst)
		eg.Go(func() error {
			if egctx.Err() != nil {
				return egctx.Err()
			}
			return cc.xcopy(dst, src)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Pass 2: copy/delete files and directories. Iteration order:
	//   RightFiles → del, RightDirs → del, Diff → copy, LeftFiles → copy.
	// All share one errgroup; the first error cancels egctx and causes
	// subsequent eg.Go calls to return egctx.Err() early.
	eg, egctx = errgroup.WithContext(ctx)
	eg.SetLimit(conc)

	submit := func(p string, fn func() error) bool {
		if egctx.Err() != nil {
			return false
		}
		eg.Go(func() error {
			if egctx.Err() != nil {
				return egctx.Err()
			}
			return fn()
		})
		return true
	}

	cc.RightFiles.Range(func(_ string, fi *fio.Info) bool {
		nm := fi.Path()
		cc.o.Delete(nm)
		return submit(nm, func() error { return cc.doDel(nm) })
	})

	cc.RightDirs.Range(func(_ string, fi *fio.Info) bool {
		nm := fi.Path()
		cc.o.Delete(nm)
		return submit(nm, func() error { return cc.doDel(nm) })
	})

	cc.Diff.Range(func(_ string, p fio.Pair) bool {
		src := p.Src.Path()
		dst := p.Dst.Path()
		if linked := cc.h.track(p.Src, dst); linked {
			return egctx.Err() == nil
		}
		cc.o.Copy(dst, src)
		return submit(dst, func() error { return cc.doCopy(dst, src) })
	})

	cc.LeftFiles.Range(func(nm string, fi *fio.Info) bool {
		src := filepath.Join(cc.Src, nm)
		dst := filepath.Join(cc.Dst, nm)
		if linked := cc.h.track(fi, dst); linked {
			return egctx.Err() == nil
		}
		cc.o.Copy(dst, src)
		return submit(dst, func() error { return cc.doCopy(dst, src) })
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	// Pass 3: resolved hardlinks.
	eg, egctx = errgroup.WithContext(ctx)
	eg.SetLimit(conc)
	cc.h.hardlinks(func(d, s string) {
		if egctx.Err() != nil {
			return
		}
		cc.o.Link(d, s)
		eg.Go(func() error {
			if egctx.Err() != nil {
				return egctx.Err()
			}
			return cc.doLink(d, s)
		})
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	// fixup mtimes of modified dst dirs
	dirmap := make(map[string]bool)
	cc.dirs.Range(func(k string, _ bool) bool {
		dirmap[k] = true
		return true
	})
	return cc.fixup(dirmap)
}

// fixup dst dirs - esp their mtimes; the files would've been written in
// random order
func (cc *dircloner) fixup(dmap map[string]bool) error {
	// reverse sort the list of dirs so we touch the deepest
	// part of the tree first.
	dlist := _Keys(dmap)
	slices.SortFunc(dlist, func(a, b string) int {
		return strings.Compare(b, a)
	})

	// *Sequentially* Apply the MD updates - to ensure we cover all
	// child MD updates before updating the parent.
	var errs []error
	for _, p := range dlist {
		nm, _ := filepath.Rel(cc.Dst, p)
		if nm == "." {
			continue
		}

		// We have to use the latest timestamp rather than the
		// one we have in cc.Difference.Lhs.
		src := filepath.Join(cc.Src, nm)
		fi, err := fio.Lstat(src)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, &Error{"fixup", cc.Src, cc.Dst, err})
			}
			continue
		}

		if err := updateMeta(p, fi); err != nil {
			errs = append(errs, &Error{"fixup", cc.Src, cc.Dst, err})
			continue
		}
		cc.o.MetadataUpdate(p, src)
	}

	if len(errs) > 0 {
		return &Error{"fixup", cc.Src, cc.Dst, errors.Join(errs...)}
	}
	return nil
}

func _Keys[M ~map[K]V, K comparable, V any](m M) []K {
	v := make([]K, 0, len(m))
	for k := range m {
		v = append(v, k)
	}
	return v
}

// track records a dst-dir as modified. fixup later walks these to
// update dir mtimes in deepest-first order.
func (cc *dircloner) track(p string) {
	cc.dirs.Store(filepath.Dir(p), true)
}

func (cc *dircloner) doCopy(dst, src string) error {
	if err := cc.xcopy(dst, src); err != nil {
		return err
	}
	cc.track(dst)
	return nil
}

func (cc *dircloner) doDel(name string) error {
	if err := os.RemoveAll(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &Error{"rm", cc.Src, cc.Dst, err}
	}
	// NOTE: historical WorkPool implementation tracked
	// filepath.Dir(filepath.Dir(name)) here (grandparent). Preserved
	// intentionally - behavior change belongs in a separate commit.
	cc.track(filepath.Dir(name))
	return nil
}

func (cc *dircloner) doLink(dst, src string) error {
	_ = os.Remove(dst) // XXX There is no way to overwrite?
	if err := os.Link(src, dst); err != nil {
		return &Error{"ln", cc.Src, cc.Dst, err}
	}
	cc.track(dst)
	return nil
}

// take a list of paths and return only longest prefixes
func dirlist(m *fio.Map) []string {
	if m.Size() == 0 {
		return []string{}
	}

	keys := make([]string, 0, m.Size())
	m.Range(func(nm string, _ *fio.Info) bool {
		keys = append(keys, nm)
		return true
	})

	slices.Sort(keys)
	return keys
}

// for now this is unused
func longestPrefixes(keys []string) []string {
	slices.Sort(keys)

	// now iterate through the array and find the longest prefixes
	dirs := keys[:0]
	cur := keys[0]
	for _, nm := range keys[1:] {
		if len(nm) >= len(cur) && nm[0:len(cur)] == cur {
			cur = nm
		} else {
			// entirely different item, output this and
			// reset
			dirs = append(dirs, cur)
			cur = nm
		}
	}
	dirs = append(dirs, cur)
	return dirs
}

func newFunnyError(m *fio.PairMap) *FunnyError {
	var f []FunnyEntry

	m.Range(func(nm string, p fio.Pair) bool {
		f = append(f, FunnyEntry{nm, p.Src, p.Dst})
		return true
	})

	return &FunnyError{f}
}

// NopObserver implements Observer and throws away all input.
// ie it's a no-op
func NopObserver() Observer {
	return &dummyObserver{}
}

type dummyObserver struct{}

var _ Observer = &dummyObserver{}

func (d *dummyObserver) Difference(_ *cmp.Difference) {}
func (d *dummyObserver) Mkdir(_ string)               {}
func (d *dummyObserver) Copy(_, _ string)             {}
func (d *dummyObserver) Delete(_ string)              {}
func (d *dummyObserver) Link(_, _ string)             {}
func (d *dummyObserver) MetadataUpdate(_, _ string)   {}
func (d *dummyObserver) VisitSrc(_ *fio.Info)         {}
func (d *dummyObserver) VisitDst(_ *fio.Info)         {}
