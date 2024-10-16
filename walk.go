// walk.go - concurrent fs-walker
//
// (c) 2022- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

// Package walk does a concurrent file system traversal and returns
// each entry. Callers can filter the returned entries via `Options` or
// a caller provided `Filter` function. This library uses all the available
// CPUs (as returned by `runtime.NumCPU()`) to maximize concurrency of the
// file tree traversal.
//
// This library can detect mount point crossings, follow symlinks and also
// return extended attributes (xattr(7)).
package fio

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// High level design:
//
// * multiple workers; each worker is responsible for processing a single
//   directory and its contents. A worker *always* outputs the directory entry
//   before descending to its children.
// * each directory encountered bumps up a WaitGroup count (walkState::dirWg).
// * Some filtering is done when we output via the `.output()` method and
//   some filtering happens when we process entries from a directory.

// Type describes the type of a given file system entry.
type Type uint

const (
	FILE    Type = 1 << iota // regular file
	DIR                      // directory
	SYMLINK                  // symbolic link
	DEVICE                   // device special file (blk and char)
	SPECIAL                  // other special files

	// This is a short cut for "give me all entries"
	ALL = FILE | DIR | SYMLINK | DEVICE | SPECIAL
)

// Options control the behavior of the filesystem walk.
type Options struct {
	// Number of go-routines to use; if not set (ie 0),
	// Walk() will use the max available cpus
	Concurrency int

	// Follow symlinks if set
	FollowSymlinks bool

	// stay within the same file-system
	OneFS bool

	// Types of entries to return
	Type Type

	// Excludes is a list of shell-glob patterns to exclude from
	// the walk. If a dir matches the prefix, go-walk does
	// not descend that subdirectory. The matching is done on the basename
	// component of the relative pathname.
	Excludes []string

	// Filter is an optional caller provided callback
	// This function must return True if this entry should
	// no longer be processed. ie filtered out. 'nm' is the full
	// relative path (not just the basename)
	Filter func(fi *Info) bool
}

// internal state
type walkState struct {
	Options
	ch    chan string
	out   chan *Info
	errch chan error

	// type mask for output filtering
	typ os.FileMode

	// Tracks completion of the DFS walk across directories.
	// Each counter in this waitGroup tracks one subdir
	// we've encountered.
	dirWg sync.WaitGroup

	// Tracks worker goroutines
	wg sync.WaitGroup

	singlefs func(fi *Info) bool

	// the output action - either send info via chan or call user supplied func
	apply func(fi *Info)

	// Tracks device major:minor to detect mount-point crossings
	fs  sync.Map
	ino sync.Map
}

// mapping our types to the stdlib types
var typMap = map[Type]os.FileMode{
	FILE:    0,
	DIR:     os.ModeDir,
	SYMLINK: os.ModeSymlink,
	DEVICE:  os.ModeDevice | os.ModeCharDevice,
	SPECIAL: os.ModeNamedPipe | os.ModeSocket,
}

var strMap = map[Type]string{
	FILE:    "File",
	DIR:     "Dir",
	SYMLINK: "Symlink",
	DEVICE:  "Device",
	SPECIAL: "Special",
}

// Stringer for walk filter Type
func (t Type) String() string {
	var z []string
	for k, v := range strMap {
		if (k & t) > 0 {
			z = append(z, v)
		}
	}
	return strings.Join(z, "|")
}

// Walk traverses the entries in 'names' in a concurrent fashion and returns
// results in a channel of *Info. The caller must service the channel. Any errors
// encountered during the walk are returned in the error channel.
func Walk(names []string, opt *Options) (chan *Info, chan error) {
	if opt.Concurrency <= 0 {
		opt.Concurrency = runtime.NumCPU()
	}

	out := make(chan *Info, opt.Concurrency)
	d := newWalkState(opt)

	// This function sends output to a chan
	d.apply = func(fi *Info) {
		out <- fi
	}

	d.doWalk(names)

	// close the channels when we're all done
	go func() {
		d.dirWg.Wait()
		close(d.ch)
		close(out)
		close(d.errch)
		d.wg.Wait()
	}()

	return out, d.errch
}

// WalkFunc traverses the entries in 'names' in a concurrent fashion and calls 'apply'
// for entries that match criteria in 'opt'. The apply function must be concurrency-safe
// ie it will be called concurrently from multiple go-routines. Any errors reported by
// 'apply' will be returned from WalkFunc().
func WalkFunc(names []string, opt *Options, apply func(fi *Info) error) error {
	if opt.Concurrency <= 0 {
		opt.Concurrency = runtime.NumCPU()
	}

	d := newWalkState(opt)

	// This calls the caller supplied 'apply' func
	d.apply = func(fi *Info) {
		if err := apply(fi); err != nil {
			d.errch <- err
		}
	}

	d.doWalk(names)

	// harvest errors and prepare to return
	var errWg sync.WaitGroup
	var errs []error

	errWg.Add(1)
	go func(in chan error) {
		for e := range in {
			errs = append(errs, e)
		}
		errWg.Done()
	}(d.errch)

	// close the channels when we're all done
	d.dirWg.Wait()
	close(d.ch)
	close(d.errch)
	errWg.Wait()
	d.wg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func newWalkState(opt *Options) *walkState {
	if opt == nil {
		opt = &Options{}
	}

	d := &walkState{
		Options: *opt,
		ch:      make(chan string, opt.Concurrency),
		errch:   make(chan error, opt.Concurrency),
		singlefs: func(_ *Info) bool {
			return true
		},
	}
	return d
}

// walk the entries in 'names'; this creates workers to
// traverse the FS in a concurrent fashion.
func (d *walkState) doWalk(names []string) {
	if d.OneFS {
		d.singlefs = d.isSingleFS
	}

	// default accept filter
	if d.Filter == nil {
		// by default - "don't filter anything"
		d.Filter = func(_ *Info) bool {
			return false
		}
	}

	// build a fast lookup of our types to stdlib
	t := d.Type
	for k, v := range typMap {
		if (t & k) > 0 {
			d.typ |= v
		}
	}

	d.wg.Add(d.Concurrency)
	for i := 0; i < d.Concurrency; i++ {
		go d.worker()
	}

	// send work to workers
	dirs := make([]string, 0, len(names))
	for i := range names {
		nm := strings.TrimSuffix(names[i], "/")
		if len(nm) == 0 {
			nm = "/"
		}

		if d.exclude(nm) {
			continue
		}

		fi := d.newInfo()
		if err := Lstatm(nm, fi); err != nil {
			d.error("lstat %s: %w", nm, err)
			continue
		}

		// don't process entries we've already seen
		if d.isEntrySeen(fi) {
			continue
		}

		if d.Filter(fi) {
			continue
		}

		m := fi.Mode()
		switch {
		case m.IsDir():
			if d.OneFS {
				d.trackFS(fi)
			}
			dirs = append(dirs, nm)

		case (m & os.ModeSymlink) > 0:
			// we may have new info now. The symlink may point to file, dir or
			// special.
			dirs = d.doSymlink(fi, dirs)

		default:
			d.output(fi)
		}
	}

	// queue the dirs
	d.enq(dirs)

}

// worker thread to walk directories
func (d *walkState) worker() {
	for nm := range d.ch {
		fi := d.newInfo()
		if err := Lstatm(nm, fi); err != nil {
			d.error("lstat %s: %w", nm, err)
			d.dirWg.Done()
			continue
		}

		// we are _sure_ this is a dir.
		d.output(fi)

		// Now process the contents of this dir
		d.walkPath(nm)

		// It is crucial that we do this as the last thing in the processing loop.
		// Otherwise, we have a race condition where the workers will prematurely quit.
		// We can only decrement this wait-group _after_ walkPath() has returned!
		d.dirWg.Done()
	}

	d.wg.Done()
}

// output action for entries we encounter
func (d *walkState) output(fi *Info) {
	//fmt.Printf("out: %s\n", fi.Name())
	m := fi.Mode()

	// we have to special case regular files because there is
	// no mask for Regular Files!
	//
	// For everyone else, we can consult the typ map
	if (d.typ&m) > 0 || ((d.Type&FILE) > 0 && m.IsRegular()) {
		d.apply(fi)
	}
}

// return true iff basename(nm) matches one of the patterns
func (d *walkState) exclude(nm string) bool {
	if len(d.Excludes) == 0 {
		return false
	}

	bn := path.Base(nm)
	for _, pat := range d.Excludes {
		ok, err := path.Match(pat, bn)
		if err != nil {
			d.errch <- fmt.Errorf("glob '%s': %s", pat, err)
		} else if ok {
			return true
		}
	}

	return false
}

// enqueue a list of dirs in a separate go-routine so the caller is
// not blocked (deadlocked)
func (d *walkState) enq(dirs []string) {
	if len(dirs) > 0 {
		d.dirWg.Add(len(dirs))
		go func(dirs []string) {
			for _, nm := range dirs {
				d.ch <- nm
			}
		}(dirs)
	}
}

// read a dir and return the names
func readDir(nm string) ([]string, error) {
	fd, err := os.Open(nm)
	if err != nil {
		return nil, fmt.Errorf("readdir: %s: %w", nm, err)
	}
	defer fd.Close()

	names, err := fd.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("readdir: %s: %w", nm, err)
	}
	return names, nil
}

// Process a directory and return the list of subdirs
//
// There is *no* race condition between the workers reading d.ch and the
// wait-group going to zero: there is at least 1 count outstanding: of the
// current entry being processed. So, this function can take as long as it wants
// the caller (d.worker()) won't decrement that wait-count until this function
// returns. And by then the wait-count would've been bumped up by the number of
// dirs we've seen here.
func (d *walkState) walkPath(nm string) {
	names, err := readDir(nm)
	if err != nil {
		d.error("%w", err)
		return
	}

	// hack to make joined paths not look like '//file'
	if nm == "/" {
		nm = ""
	}

	dirs := make([]string, 0, len(names)/2)
	for i := range names {
		entry := names[i]

		// we don't want to use filepath.Join() because it "cleans"
		// the path (removes the leading .)
		fp := fmt.Sprintf("%s/%s", nm, entry)

		fi := d.newInfo()
		err := Lstatm(fp, fi)
		if err != nil {
			d.error("walk: stat %s: %w", fp, err)
			continue
		}

		m := fi.Mode()
		if d.exclude(fp) {
			continue
		}

		// don't process entries we've already seen
		if d.isEntrySeen(fi) {
			continue
		}

		if d.Filter(fi) {
			continue
		}

		switch {
		case m.IsDir():
			// don't descend if this directory is not on the same file system.
			if d.singlefs(fi) {
				dirs = append(dirs, fp)
			}

		case (m & os.ModeSymlink) > 0:
			// we may have new info now. The symlink may point to file, dir or
			// special.
			dirs = d.doSymlink(fi, dirs)

		default:
			d.output(fi)
		}
	}

	d.enq(dirs)
}

// Walk symlinks and don't process dirs/entries that we've already seen
// This function updates dirs if the resolved symlink is a dir we have
// to descend - and returns the possibly updated dirs list.
func (d *walkState) doSymlink(fi *Info, dirs []string) []string {
	if !d.FollowSymlinks {
		d.output(fi)
		return dirs
	}

	// process symlinks until we are done
	nm := fi.Name()
	newnm, err := filepath.EvalSymlinks(nm)
	if err != nil {
		d.error("symlink %s: %s", nm, err)
		return dirs
	}
	nm = newnm

	// we know this is no longer a symlink
	if err = Statm(nm, fi); err != nil {
		d.error("symlink: stat %s: %s", nm, err)
		return dirs
	}

	// do rest of processing iff we haven't seen this entry before.
	if !d.isEntrySeen(fi) {
		switch {
		case fi.Mode().IsDir():
			// we only have to worry about mount points
			if d.singlefs(fi) {
				dirs = append(dirs, nm)
			}
		default:
			d.output(fi)
		}
	}

	return dirs
}

// track this inode to detect loops; return true if we've seen it before
// false otherwise.
func (d *walkState) isEntrySeen(st *Info) bool {
	key := fmt.Sprintf("%d:%d:%d", st.Dev, st.Rdev, st.Ino)
	x, ok := d.ino.LoadOrStore(key, st)
	if !ok {
		return false
	}

	// This can't fail because we checked it above before storing in the
	// sync.Map
	xt := x.(*Info)

	//fmt.Printf("# %s: old ino: %d:%d:%d  <-> new ino: %d:%d:%d\n", nm, xt.Dev, xt.Rdev, xt.Ino, st.Dev, st.Rdev, st.Ino)

	if xt.Dev != st.Dev || xt.Rdev != st.Rdev || xt.Ino != st.Ino {
		return false
	}

	return true
}

// track this file for future mount points
// We call this function once for each entry passed to Walk().
func (d *walkState) trackFS(fi *Info) {
	key := fmt.Sprintf("%d:%d", fi.Dev, fi.Rdev)
	d.fs.Store(key, fi)
}

// Return true if the inode is on the same file system as the command line args
func (d *walkState) isSingleFS(fi *Info) bool {
	key := fmt.Sprintf("%d:%d", fi.Dev, fi.Rdev)
	if _, ok := d.fs.Load(key); ok {
		return true
	}
	return false
}

// enq an error
func (d *walkState) error(s string, args ...any) {
	d.errch <- fmt.Errorf(s, args...)
}

// TODO mem pool for info
func (d *walkState) newInfo() *Info {
	return new(Info)
}

// EOF
