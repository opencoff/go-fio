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
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/walk"
	"github.com/puzpuzpuz/xsync/v3"
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

func (f IgnoreFlag) String() string {
	var z []string
	if f&IGN_UID > 0 {
		z = append(z, "uid")
	}
	if f&IGN_GID > 0 {
		z = append(z, "gid")
	}
	if f&IGN_HARDLINK > 0 {
		z = append(z, "links")
	}
	if f&IGN_XATTR > 0 {
		z = append(z, "xattr")
	}

	return strings.Join(z, ",")
}

type cmpopt struct {
	walk.Options

	deepEq func(lhs, rhs *fio.Info) bool

	// file-sys attributes to ignore for equality comparison
	// Used by cmp.DirCmp
	ignoreAttr IgnoreFlag
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

type cmp struct {
	cmpopt

	src, dst string

	cache *statCache

	fileEq fileqFunc

	lhsDir  *FioMap
	lhsFile *FioMap
	rhsDir  *FioMap
	rhsFile *FioMap

	commonDir  *FioPairMap
	commonFile *FioPairMap

	diff *FioPairMap

	funny *FioPairMap

	done *xsync.MapOf[string, bool]
}

// Pair represents the Stat/Lstat info of a pair of
// related file system entries in the source and destination
type Pair struct {
	Src, Dst *fio.Info
}

// FioMap is a concurrency safe map of relative path name and the
// corresponding Stat/Lstat info.
type FioMap = xsync.MapOf[string, *fio.Info]

// FioPairMap is a concurrency safe map of relative path name and the
// corresponding Stat/Lstat info of both the source and destination.
type FioPairMap = xsync.MapOf[string, Pair]

func newMap() *FioMap {
	return xsync.NewMapOf[string, *fio.Info]()
}

func newPairMap() *FioPairMap {
	return xsync.NewMapOf[string, Pair]()
}

// DirTree compares two directory trees 'src' and 'dst'.  For regular files,
// it compares file size and mtime to determine change.
// For all entries, it compares every comparable attribute of fio.Info - unless
// explicitly ignored (by using the option WithIgnore()).
func DirTree(src, dst string, opt ...Option) (*Difference, error) {
	option := defaultOptions()

	for _, fp := range opt {
		fp(&option)
	}

	c, err := newCmp(src, dst, &option)
	if err != nil {
		return nil, err
	}

	if err = c.gatherSrc(); err != nil {
		return nil, err
	}

	if err = c.gatherDst(); err != nil {
		return nil, err
	}

	// now we have differences - pull them together
	d := &Difference{
		Src: src,
		Dst: dst,

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
	c.cache.Clear()
	c.done.Clear()

	return d, nil
}

// Difference captures the results of comparing two directory trees
type Difference struct {
	Src string
	Dst string

	LeftDirs   *FioMap
	LeftFiles  *FioMap
	RightDirs  *FioMap
	RightFiles *FioMap

	CommonDirs  *FioPairMap
	CommonFiles *FioPairMap

	Diff  *FioPairMap
	Funny *FioPairMap
}

func (d *Difference) String() string {
	var b strings.Builder
	d1 := func(desc string, m *FioMap) {
		fmt.Fprintf(&b, "%s:\n", desc)
		m.Range(func(nm string, fi *fio.Info) bool {
			fmt.Fprintf(&b, "\t%s: %s\n", nm, fi)
			return true
		})
	}

	d2 := func(desc string, m *FioPairMap) {
		fmt.Fprintf(&b, "%s:\n", desc)
		m.Range(func(nm string, p Pair) bool {
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

// clone src to dst; we know both are dirs
func newCmp(src, dst string, opt *cmpopt) (*cmp, error) {
	lhs, err := fio.Lstat(src)
	if err != nil {
		return nil, &Error{"lstat-src", src, dst, err}
	}

	if !lhs.IsDir() {
		return nil, &Error{"source not a dir", src, dst, nil}
	}

	rhs, err := fio.Lstat(dst)
	if err != nil {
		return nil, &Error{"lstat-dst", src, dst, err}
	}

	if !rhs.IsDir() {
		return nil, &Error{"destination not a dir", src, dst, nil}
	}

	c := &cmp{
		cmpopt: *opt,
		src:    src,
		dst:    dst,
		cache:  newStatCache(),

		fileEq: makeEqFunc(opt),

		// the map-value for each of these is the lhs fio.Info
		lhsDir:  newMap(),
		lhsFile: newMap(),
		rhsDir:  newMap(),
		rhsFile: newMap(),

		commonDir:  newPairMap(),
		commonFile: newPairMap(),
		diff:       newPairMap(),
		funny:      newPairMap(),

		done: xsync.NewMapOf[string, bool](),
	}

	return c, nil
}

// walk the lhs (ie src) and gather all the contents in the appropriate
// maps.
func (c *cmp) gatherSrc() error {
	// we will add a pre-filter for the tree traversal
	// so we don't descend certain dirs
	filter := func(fi *fio.Info) (bool, error) {
		// walk always uses Lstat for the filter invocation
		c.cache.StoreLstat(fi)

		if !fi.IsDir() {
			return false, nil
		}

		nm, _ := filepath.Rel(c.src, fi.Name())
		if nm == "." {
			return false, nil
		}

		dst := filepath.Join(c.dst, nm)
		if rhs, err := c.cache.Lstat(dst); err == nil {
			// if rhs is NOT a dir, then it's a conflict ("funny files")
			// And we can skip further entries from the lhs
			if !rhs.IsDir() {
				c.funny.Store(nm, Pair{fi, rhs})
				return true, nil
			}
		}

		// continue processing this entry
		return false, nil
	}

	// make a local copy; we will need a clean version for gathering rhs later on.
	wo := c.cmpopt.Options
	wo.Filter = filter

	if err := walk.WalkFunc([]string{c.src}, wo, c.processLhs); err != nil {
		return &Error{"walk-src", c.src, c.dst, err}
	}

	return nil
}

// process one entry from lhs.
func (c *cmp) processLhs(lhs *fio.Info) error {
	c.cache.StoreLstat(lhs)
	nm, _ := filepath.Rel(c.src, lhs.Name())
	if nm == "." {
		return nil
	}

	src := lhs.Name()
	dst := filepath.Join(c.dst, nm)
	rhs, err := c.cache.Lstat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			if lhs.IsDir() {
				c.lhsDir.Store(nm, lhs)
			} else {
				c.lhsFile.Store(nm, lhs)
			}
			return nil
		}

		// report other errors
		return &Error{"lstat-dst", src, dst, err}
	}

	// in all cases below - we will store these two together
	// in the appropriate map
	pair := Pair{lhs, rhs}

	// if the file types don't match - skip
	if (lhs.Mod & ^fs.ModePerm) != (rhs.Mod & ^fs.ModePerm) {
		c.funny.Store(nm, pair)
		return nil
	}

	// mark this common entry as processed
	c.done.Store(nm, true)

	// both are same "type" of files
	if lhs.IsRegular() {
		// compare file sizes and mark differences
		if lhs.Size() != rhs.Size() {
			c.diff.Store(nm, pair)
			return nil
		}
	}

	// all other "types" - we will compare attributes
	if eq, _ := c.fileEq(lhs, rhs); !eq {
		c.diff.Store(nm, pair)
		return nil
	}

	if lhs.IsDir() {
		c.commonDir.Store(nm, pair)
	} else {
		c.commonFile.Store(nm, pair)
	}
	return nil
}

// walk rhs (ie dst) and gather entries that are unique to the rhs; the other
// entries that are shared are already handled in gatherSrc() above.
func (c *cmp) gatherDst() error {
	// reset the filter for this walk
	wo := c.cmpopt.Options

	err := walk.WalkFunc([]string{c.dst}, wo, func(rhs *fio.Info) error {
		nm, _ := filepath.Rel(c.dst, rhs.Name())
		if nm == "." {
			return nil
		}

		if _, ok := c.done.Load(nm); ok {
			return nil
		}

		if _, ok := c.funny.Load(nm); ok {
			return nil
		}

		// this is an entry that is ONLY on the rhs
		// otherwise, we'd have processed it previously when we
		// walked lhs above.
		if rhs.IsDir() {
			c.rhsDir.Store(nm, rhs)
		} else {
			c.rhsFile.Store(nm, rhs)
		}
		return nil
	})

	if err != nil {
		return &Error{"walk-dst", c.src, c.dst, err}
	}

	return nil
}


type diffType uint

const (
	_D_MTIME diffType = 1 << iota
	_D_UID
	_D_GID
	_D_LINK
	_D_XATTR
	_D_CUSTOM
)

var diffTypeName map[diffType]string = map[diffType]string{
	_D_MTIME:  "mtime",
	_D_UID:    "uid",
	_D_GID:    "gid",
	_D_LINK:   "link",
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
	if !ignore(IGN_HARDLINK) {
		eqv = append(eqv, func(lhs, rhs *fio.Info) (bool, diffType) {
			return lhs.Nlink == rhs.Nlink, _D_LINK
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
