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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
)

type Option func(o *treeopt)

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
		o.walkopt = wo
	}
}

type treeopt struct {
	fl cmp.IgnoreFlag

	walkopt walk.Options
}

func defaultOptions() treeopt {
	opt := treeopt{
		walkopt: walk.Options{
			Concurrency: runtime.NumCPU(),
			Type:        walk.ALL,
		},
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
		if !os.IsNotExist(err) {
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

	diff, err := cmp.DirTree(src, dst, cmp.WithIgnoreAttr(option.fl),
		cmp.WithWalkOptions(option.walkopt))
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
	*cmp.Difference

	ncpu int

	ech chan error
	och chan work
	wg  sync.WaitGroup

	// sharded dirs that are modified
	dirs []map[string]bool
}

func newCloner(d *cmp.Difference, opt *treeopt) *dircloner {
	ncpu := opt.walkopt.Concurrency

	cc := &dircloner{
		Difference: d,
		ncpu:       ncpu,
		ech:        make(chan error, 1),
		och:        make(chan work, ncpu),
		dirs:       make([]map[string]bool, ncpu),
	}

	for i := 0; i < ncpu; i++ {
		cc.dirs[i] = make(map[string]bool, 8)
	}

	return cc
}

func (cc *dircloner) clone() error {

	// each worker will track the dirs they modify in a sharded map
	// the shards will be combined later
	wp := fio.NewWorkPool[work](cc.ncpu, func(i int, w work) error {
		var err error
		cc.dirs[i], err = cc.dowork(cc.dirs[i], w)
		return err
	})

	// now submit work to the workpool

	// LeftDirs => new dirs in dst
	// LeftFiles => copy to new dirs
	// Diff => overwrite + COW src to dst
	// RightFiles -- delete first
	// RightDirs -- delete last

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		cc.RightFiles.Range(func(_ string, fi *fio.Info) bool {
			wp.Submit(&delOp{fi.Name()})
			return true
		})
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cc.RightDirs.Range(func(_ string, fi *fio.Info) bool {
			wp.Submit(&delOp{fi.Name()})
			return true
		})
		wg.Done()
	}()

	// now submit copies
	wg.Add(1)
	go func() {
		cc.Diff.Range(func(_ string, p cmp.Pair) bool {
			wp.Submit(&copyOp{p.Src.Name(), p.Dst.Name()})
			return true
		})
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		cc.LeftFiles.Range(func(nm string, _ *fio.Info) bool {
			src := filepath.Join(cc.Src, nm)
			dst := filepath.Join(cc.Dst, nm)
			wp.Submit(&copyOp{src, dst})
			return true
		})
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		// this clones dirs
		cc.LeftDirs.Range(func(nm string, _ *fio.Info) bool {
			src := filepath.Join(cc.Src, nm)
			dst := filepath.Join(cc.Dst, nm)
			wp.Submit(&copyOp{src, dst})
			return true
		})
		wg.Done()
	}()

	// submit all the work and then tell workers we're done
	go func() {
		wg.Wait()
		wp.Close()
	}()

	err := wp.Wait()
	if err != nil {
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
	wp := fio.NewWorkPool[copyOp](cc.ncpu, func(_ int, w copyOp) error {
		return File(w.dst, w.src)
	})

	for p := range dmap {
		nm, _ := filepath.Rel(cc.Dst, p)
		if nm != "." {
			src := filepath.Join(cc.Src, nm)
			wp.Submit(copyOp{src, p})
		}
	}

	wp.Close()

	return wp.Wait()
}

func (cc *dircloner) dowork(dirs map[string]bool, w work) (map[string]bool, error) {
	track := func(p string) {
		dn := filepath.Dir(p)
		dirs[dn] = true
	}

	switch z := w.(type) {
	case *copyOp:
		if err := File(z.dst, z.src); err != nil {
			return dirs, err
		}
		track(z.dst)

	case *delOp:
		err := os.RemoveAll(z.name)
		if err != nil && !os.IsNotExist(err) {
			return dirs, &Error{"rm", cc.Src, cc.Dst, err}
		}
		track(z.name)
	default:
		err := fmt.Errorf("unknown op %T", w)
		return dirs, &Error{"clone", cc.Src, cc.Dst, err}
	}
	return dirs, nil
}

type work any

type copyOp struct {
	src, dst string
}

type delOp struct {
	name string
}

func newFunnyError(m *cmp.FioPairMap) *FunnyError {
	var f []FunnyEntry

	m.Range(func(nm string, p cmp.Pair) bool {
		f = append(f, FunnyEntry{nm, p.Src, p.Dst})
		return true
	})

	return &FunnyError{f}
}
