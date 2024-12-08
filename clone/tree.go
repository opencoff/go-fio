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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
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
func Tree(dst, src string, opt ...Option) error {
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

	diff, err := cmp.FsTree(src, dst, cmp.WithIgnoreAttr(option.fl),
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

	if err = n.clone(); err != nil {
		return err
	}

	return nil
}

type dircloner struct {
	treeopt

	*cmp.Difference

	h *hardlinker

	// sharded dirs that are modified
	dirs []map[string]bool
}

func newCloner(d *cmp.Difference, opt *treeopt) *dircloner {
	ncpu := opt.Concurrency

	cc := &dircloner{
		treeopt:    *opt,
		Difference: d,
		h:          newHardlinker(),
		dirs:       make([]map[string]bool, ncpu),
	}

	for i := 0; i < ncpu; i++ {
		cc.dirs[i] = make(map[string]bool, 8)
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

func (cc *dircloner) clone() error {
	// first make the new dirs before attempting to make files.
	// We need to do this first before we copy over any new files.
	dirs := dirlist(cc.LeftDirs)
	dirWp := fio.NewWorkPool[copyOp](cc.Concurrency, func(_ int, w copyOp) error {
		return cc.xcopy(w.dst, w.src)
	})

	dm := cc.dirs[0]
	for _, nm := range dirs {
		src := filepath.Join(cc.Src, nm)
		dst := filepath.Join(cc.Dst, nm)

		dm[dst] = true
		dirWp.Submit(copyOp{src, dst})
		cc.o.Mkdir(dst)
	}
	dirWp.Close()
	if err := dirWp.Wait(); err != nil {
		return err
	}

	// now start copying and deleting files
	// each worker will track the dirs they modify in a sharded map
	// the shards will be combined later

	wp := fio.NewWorkPool[work](cc.Concurrency, func(i int, w work) error {
		var err error
		cc.dirs[i], err = cc.dowork(cc.dirs[i], w)
		return err
	})

	// now submit work to the workpool

	// LeftFiles => copy to new dirs
	// Diff => overwrite + COW src to dst
	// RightFiles -- delete first
	// RightDirs -- delete last

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		cc.RightFiles.Range(func(_ string, fi *fio.Info) bool {
			wp.Submit(&delOp{fi.Name()})
			cc.o.Delete(fi.Name())
			return true
		})
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cc.RightDirs.Range(func(_ string, fi *fio.Info) bool {
			wp.Submit(&delOp{fi.Name()})
			cc.o.Delete(fi.Name())
			return true
		})
		wg.Done()
	}()

	// now submit copies
	wg.Add(1)
	go func() {
		cc.Diff.Range(func(_ string, p fio.Pair) bool {
			src := p.Src.Name()
			dst := p.Dst.Name()

			if linked := cc.h.track(p.Src, dst); !linked {
				wp.Submit(&copyOp{src, dst})
				cc.o.Copy(dst, src)
			}
			return true
		})
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cc.LeftFiles.Range(func(nm string, fi *fio.Info) bool {
			src := filepath.Join(cc.Src, nm)
			dst := filepath.Join(cc.Dst, nm)

			if linked := cc.h.track(fi, dst); !linked {
				wp.Submit(&copyOp{src, dst})
				cc.o.Copy(dst, src)
			}
			return true
		})
		wg.Done()
	}()

	// submit all the work and then tell workers we're done
	wg.Wait()
	wp.Close()
	if err := wp.Wait(); err != nil {
		return err
	}

	// now complete the pending hardlinks
	wp = fio.NewWorkPool[work](cc.Concurrency, func(i int, w work) error {
		var err error
		cc.dirs[i], err = cc.dowork(cc.dirs[i], w)
		return err
	})

	wg.Add(1)
	go func() {
		cc.h.hardlinks(func(d, s string) {
			wp.Submit(&linkOp{s, d})
			cc.o.Link(d, s)
		})
		wg.Done()
	}()

	wg.Wait()
	wp.Close()
	if err := wp.Wait(); err != nil {
		return err
	}

	// merge the various dir shards into a single one
	dirmap := cc.dirs[0]
	for _, dirs := range cc.dirs[1:] {
		for nm := range dirs {
			dirmap[nm] = true
		}
	}

	// fixup mtimes of modified dirs
	return cc.fixup(dirmap)
}

// fixup dst dirs - esp their mtimes; the files would've been written in
// random order
func (cc *dircloner) fixup(dmap map[string]bool) error {
	// clone dir metadata of modified dirs
	wp := fio.NewWorkPool[mdOp](cc.Concurrency, func(_ int, w mdOp) error {
		if err := UpdateMetadata(w.dst, w.src); err != nil {
			if cc.ignoreMissing && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	})

	// reverse sort the list of dirs so we touch the deepest
	// part of the tree first.
	dlist := _Keys(dmap)
	slices.SortFunc(dlist, func(a, b string) int {
		if b < a {
			return -1
		}
		if b > a {
			return +1
		}
		return 0
	})

	var errs []error
	for _, p := range dlist {
		nm, _ := filepath.Rel(cc.Dst, p)
		if nm == "." {
			continue
		}

		src := filepath.Join(cc.Src, nm)
		fi, err := fio.Lstat(src)
		if err != nil {
			errs = append(errs, &Error{"fixup", cc.Src, cc.Dst, err})
			continue
		}
		wp.Submit(mdOp{fi, p})
		cc.o.MetadataUpdate(p, src)
	}

	wp.Close()
	if err := wp.Wait(); err != nil {
		errs = append(errs, err)
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

func (cc *dircloner) dowork(dirs map[string]bool, w work) (map[string]bool, error) {
	track := func(p string) {
		dn := filepath.Dir(p)
		dirs[dn] = true
	}

	switch z := w.(type) {
	case *copyOp:
		if err := cc.xcopy(z.dst, z.src); err != nil {
			return dirs, err
		}
		track(z.dst)

	case *delOp:
		err := os.RemoveAll(z.name)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return dirs, &Error{"rm", cc.Src, cc.Dst, err}
		}
		track(z.name)

	case *linkOp:
		_ = os.Remove(z.dst) // XXX There is no way to overwrite?
		if err := os.Link(z.src, z.dst); err != nil {
			return dirs, &Error{"ln", cc.Src, cc.Dst, err}
		}
		track(z.dst)
	default:
		err := fmt.Errorf("unknown op %T", w)
		return dirs, &Error{"clone", cc.Src, cc.Dst, err}
	}
	return dirs, nil
}

// take a list of paths and return only longest prefixes
func dirlist(m *fio.FioMap) []string {
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

type work any

type copyOp struct {
	src, dst string
}

type delOp struct {
	name string
}

type linkOp struct {
	src, dst string
}

type mdOp struct {
	src *fio.Info
	dst string
}

func newFunnyError(m *fio.FioPairMap) *FunnyError {
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
