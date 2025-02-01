// cmp.go - compare directories
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

package cmp

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/walk"
	"github.com/puzpuzpuz/xsync/v3"
)

// IgnoreFlag captures the attributes we want to ignore while comparing
// two fio.Info instances representing two filesystem entries on the
// two trees being compared.
type IgnoreFlag uint

const (
	IGN_UID   IgnoreFlag = 1 << iota // ignore uid
	IGN_GID                          // ignore gid
	IGN_XATTR                        // ignore xattr
)

func (f IgnoreFlag) String() string {
	var z []string
	if f&IGN_UID > 0 {
		z = append(z, "uid")
	}
	if f&IGN_GID > 0 {
		z = append(z, "gid")
	}
	if f&IGN_XATTR > 0 {
		z = append(z, "xattr")
	}

	return strings.Join(z, ",")
}

type cmpopt struct {
	walk.Options

	// file-sys attributes to ignore for equality comparison
	// Used by cmp.DirCmp
	ignoreAttr IgnoreFlag

	deepEq func(lhs, rhs *fio.Info) bool

	o Observer
}

func defaultOptions() cmpopt {
	return cmpopt{
		Options: walk.Options{
			Concurrency:    runtime.NumCPU(),
			Type:           walk.ALL,
			OneFS:          false,
			FollowSymlinks: false,
			Excludes:       []string{".zfs"},
		},
		ignoreAttr: 0,
		o:          &dummyObserver{},
	}
}

// Option captures the various options for cloning
// a directory tree.
type Option func(o *cmpopt)

// WithIgnoreAttr captures the attributes of fio.Info that must be
// ignored for comparing equality of two filesystem entries.
func WithIgnoreAttr(fl IgnoreFlag) Option {
	return func(o *cmpopt) {
		o.ignoreAttr = fl
	}
}

// WithWalkOptions uses 'wo' as the option for walk.Walk(); it
// describes a caller desired traversal of the file system with
// the requisite input and output filters
func WithWalkOptions(wo walk.Options) Option {
	return func(o *cmpopt) {
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

// WithDeepCompare provides a caller supplied comparison function
// that will be invoked if all other comparable attributes are
// identical.
func WithDeepCompare(same func(lhs, rhs *fio.Info) bool) Option {
	return func(o *cmpopt) {
		o.deepEq = same
	}
}

// WithConcurrency limits the use of concurrent goroutines to n.
func WithConcurrency(n int) Option {
	return func(o *cmpopt) {
		if n <= 0 {
			n = runtime.NumCPU()
		}
		o.Concurrency = n
	}
}

// Observer is invoked when the comparator visits entries
// in src and dst.
type Observer interface {
	VisitSrc(fi *fio.Info)
	VisitDst(fi *fio.Info)
}

// WithObserver uses 'ob' to report activities as the tree
// cloner makes progress
func WithObserver(ob Observer) Option {
	return func(o *cmpopt) {
		o.o = ob
	}
}

type cmp struct {
	cmpopt

	src, dst string

	lhs, rhs *fio.FioMap

	fileEq fileqFunc

	lhsDir  *fio.FioMap
	lhsFile *fio.FioMap
	rhsDir  *fio.FioMap
	rhsFile *fio.FioMap

	commonDir  *fio.FioPairMap
	commonFile *fio.FioPairMap

	diff *fio.FioPairMap

	funny *fio.FioPairMap

	done *xsync.MapOf[string, bool]
}

// Difference captures the results of comparing two directory trees
type Difference struct {
	Src string
	Dst string

	// All the entries in the src and dst
	Lhs *fio.FioMap
	Rhs *fio.FioMap

	// Dirs that are only on the left
	LeftDirs *fio.FioMap

	// Files that are only on the left
	LeftFiles *fio.FioMap

	// Dirs that are only on the right
	RightDirs *fio.FioMap

	// Files that are only on the right
	RightFiles *fio.FioMap

	// Dirs that are identical on both sides
	CommonDirs *fio.FioPairMap

	// Files that are identical on both sides
	CommonFiles *fio.FioPairMap

	// Files/dirs that are different on both sides
	Diff *fio.FioPairMap

	// Funny entries
	Funny *fio.FioPairMap
}

func (d *Difference) String() string {
	var b strings.Builder
	d1 := func(desc string, m *fio.FioMap) {
		if m.Size() <= 0 {
			return
		}

		fmt.Fprintf(&b, "%s:\n", desc)
		m.Range(func(nm string, fi *fio.Info) bool {
			fmt.Fprintf(&b, "\t%s: %s\n", nm, fi)
			return true
		})
	}

	d2 := func(desc string, m *fio.FioPairMap) {
		if m.Size() <= 0 {
			return
		}

		fmt.Fprintf(&b, "%s:\n", desc)
		m.Range(func(nm string, p fio.Pair) bool {
			fmt.Fprintf(&b, "\t%s:\n\t\tsrc %s\n\t\tdst %s\n", nm, p.Src, p.Dst)
			return true
		})
	}

	fmt.Fprintf(&b, "---Diff Output---\nSrc: %s\nDst: %s\n", d.Src, d.Dst)

	d1("Left-only dirs", d.LeftDirs)
	d1("Left-only files", d.LeftFiles)
	d1("Right-only dirs", d.RightDirs)
	d1("Right-only files", d.RightFiles)

	d2("Common dirs", d.CommonDirs)
	d2("Common files", d.CommonFiles)

	d2("Funny files", d.Funny)
	d2("Differences", d.Diff)

	b.WriteString("---End Diff Output---\n")
	return b.String()
}

// FsTree compares two file system trees 'src' and 'dst'.  For regular files,
// it compares file size and mtime to determine change.
// For all entries, it compares every comparable attribute of fio.Info - unless
// explicitly ignored (by using the option WithIgnore()). The ignorable
// attributes are identified by IGN_xxx constants.
func FsTree(src, dst string, opt ...Option) (*Difference, error) {
	lfi, err := fio.Lstat(src)
	if err != nil {
		return nil, &Error{"lstat-src", src, dst, err}
	}

	if !lfi.IsDir() {
		return nil, &Error{"source not a dir", src, dst, nil}
	}

	rfi, err := fio.Lstat(dst)
	if err != nil {
		return nil, &Error{"lstat-dst", src, dst, err}
	}

	if !rfi.IsDir() {
		return nil, &Error{"destination not a dir", src, dst, nil}
	}

	option := defaultOptions()
	for _, fp := range opt {
		fp(&option)
	}

	// We ought to do both of these in parallel

	wo := option.Options

	// since we're doing both walks in parallel, we ensure concurrency limits
	// are honored
	wo.Concurrency = wo.Concurrency / 2

	var wg sync.WaitGroup
	var err_L, err_R error
	var lhs, rhs *fio.FioMap

	wg.Add(2)

	go func(w *sync.WaitGroup) {
		var err error

		lhs, err = walkTree(src, wo)
		if err != nil {
			err_L = &Error{"walk-src", src, dst, err}
		}
		w.Done()
	}(&wg)

	go func(w *sync.WaitGroup) {
		var err error

		rhs, err = walkTree(dst, wo)
		if err != nil {
			err_R = &Error{"walk-dst", src, dst, err}
		}
		w.Done()
	}(&wg)

	wg.Wait()
	if err_L != nil {
		return nil, err_L
	}
	if err_R != nil {
		return nil, err_R
	}

	d := cmpInternal(lhs, rhs, &option)

	d.Src = src
	d.Dst = dst
	return d, nil
}

// Diff takes two file system trees represented by 'lhs' and 'rhs', and
// generates the differences between them. It is almost identical to
// FsTree above - except Diff doesn't do any disk I/O. As a result,
// the option WithWalkOption is not useful.
func Diff(lhs, rhs *fio.FioMap, opt ...Option) (*Difference, error) {
	option := defaultOptions()
	for _, fp := range opt {
		fp(&option)
	}

	d := cmpInternal(lhs, rhs, &option)

	// NB: We don't know what the src and dst are; so we leave it
	//     empty.
	return d, nil
}

// common function to do the actual diff of the two trees.
// This is a CPU bound activity that shouldn't have any errors
func cmpInternal(lhs, rhs *fio.FioMap, opt *cmpopt) *Difference {
	c := newCmp(lhs, rhs, opt)

	// There should be no error from the plain differencing
	if err := c.doDiff(); err != nil {
		s := fmt.Sprintf("fs-diff: shouldn't cause errors: %s", err)
		panic(s)
	}

	// now we have differences - pull them together
	d := &Difference{
		Lhs: lhs,
		Rhs: rhs,

		LeftDirs:   c.lhsDir,
		LeftFiles:  c.lhsFile,
		RightDirs:  c.rhsDir,
		RightFiles: c.rhsFile,

		CommonDirs:  c.commonDir,
		CommonFiles: c.commonFile,
		Diff:        c.diff,
		Funny:       c.funny,
	}

	// we don't need this anymore. we can get rid of it.
	c.done.Clear()

	return d
}

// clone src to dst; we know both are dirs
func newCmp(lhs, rhs *fio.FioMap, opt *cmpopt) *cmp {
	c := &cmp{
		cmpopt: *opt,
		lhs:    lhs,
		rhs:    rhs,

		fileEq: makeEqFunc(opt),

		// the map-value for each of these is the lhs fio.Info
		lhsDir:  fio.NewFioMap(),
		lhsFile: fio.NewFioMap(),
		rhsDir:  fio.NewFioMap(),
		rhsFile: fio.NewFioMap(),

		commonDir:  fio.NewFioPairMap(),
		commonFile: fio.NewFioPairMap(),
		diff:       fio.NewFioPairMap(),
		funny:      fio.NewFioPairMap(),

		done: xsync.NewMapOf[string, bool](),
	}

	return c
}

// walk the dir 'nm' and return the full fs tree
func walkTree(nm string, wo walk.Options) (*fio.FioMap, error) {
	tree := fio.NewFioMap()

	err := walk.WalkFunc([]string{nm}, wo, func(fi *fio.Info) error {
		rel, _ := filepath.Rel(nm, fi.Path())
		if rel != "." {
			tree.Store(rel, fi)
		}
		return nil
	})

	return tree, err
}

// diffType captures the specific difference between two
// fs entries.
type diffType uint

const (
	_D_MTIME diffType = 1 << iota
	_D_UID
	_D_GID
	_D_XATTR
	_D_CUSTOM
)

var diffTypeName map[diffType]string = map[diffType]string{
	_D_MTIME:  "mtime",
	_D_UID:    "uid",
	_D_GID:    "gid",
	_D_XATTR:  "xattr",
	_D_CUSTOM: "custom",
}

func (d diffType) String() string {
	return diffTypeName[d]
}

type fileqFunc func(a, b *fio.Info) (bool, diffType)

// return a comparator function that is optimized for the attributes we are
// comparing
func makeEqFunc(opts *cmpopt) fileqFunc {
	ignore := func(fl IgnoreFlag) bool {
		if fl&opts.ignoreAttr > 0 {
			return true
		}
		return false
	}

	eqv := make([]fileqFunc, 0, 6)

	// We always have the most basic comparator: mtime
	eqv = append(eqv, func(lhs, rhs *fio.Info) (bool, diffType) {
		if lhs.Mode().Type() == fs.ModeSymlink {
			return true, _D_MTIME
		}
		return lhs.Mtim.Equal(rhs.Mtim), _D_MTIME
	})

	// build out the rest of optional comparators
	if !ignore(IGN_UID) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) (bool, diffType) {
			return lhs.Uid == rhs.Uid, _D_UID
		})
	}
	if !ignore(IGN_GID) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) (bool, diffType) {
			return lhs.Gid == rhs.Gid, _D_GID
		})
	}
	if !ignore(IGN_XATTR) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) (bool, diffType) {
			return lhs.Xattr.Equal(rhs.Xattr), _D_XATTR
		})
	}

	// we want potentially expensive comparisons to be done last.
	if opts.deepEq != nil {
		eqv = append(eqv, func(lhs, rhs *fio.Info) (bool, diffType) {
			return opts.deepEq(lhs, rhs), _D_CUSTOM
		})
	}

	// the final function will call each of these smol comparators
	// one after the other.
	return func(a, b *fio.Info) (bool, diffType) {
		for _, fp := range eqv {
			ok, x := fp(a, b)
			if !ok {
				return false, x
			}
		}
		return true, 0
	}
}

type dummyObserver struct{}

func (o *dummyObserver) VisitSrc(_ *fio.Info) {}
func (o *dummyObserver) VisitDst(_ *fio.Info) {}

var _ Observer = &dummyObserver{}
