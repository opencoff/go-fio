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
package walk

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/opencoff/go-fio"
)

// High level design:
//
// * multiple workers; each worker is responsible for processing a single
//   directory and its contents. A worker *always* outputs the directory entry
//   before descending to its children.
// * each directory encountered bumps up a WaitGroup count (walkState::dirWg).
// * Some filtering is done when we output via the `.output()` method and
//   some filtering happens when we process entries from a directory.
// * traversal errors are folded into the same output channel as normal
//   entries, packaged as Entry{Err: *walk.Error}. Callers drain one
//   channel and decide per-entry whether to abort or continue.
// * cancellation is cooperative via context.Context: workers check
//   ctx.Err() before each emit and at the top of each directory scan,
//   so a caller that abandons the walk can terminate it by cancelling
//   ctx without needing to drain the entry channel.

// Type is an output filter that can be bitwise OR'd. It denotes
// the types of file system entries that will be *returned* to the caller.
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

// Entry is one walk event: a stat'd file system entry plus walk-specific
// context (the depth relative to the root passed to Walk) — or a
// traversal error carrying the offending path.
//
// Roots passed to Walk are at Depth 0. Their direct children are at 1,
// and so on. Depth survives symlink resolution: a symlink followed to a
// directory retains the symlink's depth (descent through the link does
// not restart counting).
//
// fio.Info is embedded by value. All of its methods (Path, Name, Mode,
// ModTime, IsDir, ...) are directly callable on *Entry when Err is nil.
//
// Err carries traversal errors (typed as *walk.Error, which holds the
// failing Op and Name). When Err != nil the embedded Info is *undefined*:
// callers MUST check Err first and skip Info-based work. Depth is the
// best-effort depth at which the error was detected.
//
// NOTE: the *Entry pointer delivered to WalkFunc's apply callback and
// Options.Filter is only valid for the duration of the call. Callers who
// need to retain data across calls must copy the Entry value itself
// (e.g. `keep := *e`), not capture the pointer.
type Entry struct {
	fio.Info
	Depth int
	Err   error
}

// Options control the behavior of the filesystem walk.
type Options struct {
	// Number of go-routines to use; if not set (ie 0),
	// Walk() will use the max available cpus
	Concurrency int

	// Follow symlinks if set
	FollowSymlinks bool

	// stay within the same file-system
	OneFS bool

	// Ignore duplicate inodes. Turning this on
	// suppresses entries with hardlink count greater
	// than 1 - for those entries, only the first encountered
	// entry is output.
	IgnoreDuplicateInode bool

	// Types of entries to return
	Type Type

	// Excludes is a list of shell-glob patterns to exclude from
	// the file-system traversal. In a sense it is an "input filter" -
	// for example, excluded directories are not descended.
	// The matching is done on the basename component of the pathname.
	Excludes []string

	// Filter is an optional caller provided callback to similarly
	// exclude entries from further traversal.
	// This function must return True if this entry should
	// no longer be processed. ie filtered out. For directories, a
	// true return also prevents descent into that directory.
	//
	// The *Entry is only valid for the duration of the call; copy
	// the Entry value if you need to retain any of its data. Filter
	// is never invoked for error-entries (Err != nil).
	Filter func(e *Entry) (bool, error)
}

// walkItem is the unit of work queued between doWalk and workers.
type walkItem struct {
	path  string
	depth int
}

// internal state
type walkState struct {
	Options
	ctx context.Context
	ch  chan walkItem

	// type mask for output filtering
	typ os.FileMode

	// Tracks completion of the DFS walk across directories.
	// Each counter in this waitGroup tracks one subdir
	// we've encountered.
	dirWg sync.WaitGroup

	// Tracks worker goroutines
	wg sync.WaitGroup

	// functions that make our filtering easier
	filterName func(nm string) bool

	// return true if we haven't crossed mount point
	singlefs func(e *Entry) bool

	// the output action - either send entry via chan or call user supplied func.
	// apply handles both normal and error-entries; returns a non-nil error to
	// signal "stop walking" (cancellation or user-callback error).
	apply func(e *Entry) error

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
// results on a single channel of Entry values. Each Entry carries a
// stat-filled fio.Info (embedded by value) and the Depth relative to the
// root that reached it. Traversal errors are folded into the same channel
// as entries whose Err field is non-nil (typed as *walk.Error); the
// embedded Info is undefined in that case. Callers MUST check e.Err
// before touching Info.
//
// The returned channel is closed once the walk completes or ctx is
// cancelled. Cancelling ctx is the supported way to terminate a walk
// early without draining the channel.
func Walk(ctx context.Context, names []string, opt Options) <-chan Entry {
	if opt.Concurrency <= 0 {
		opt.Concurrency = runtime.NumCPU()
	}

	out := make(chan Entry, opt.Concurrency)
	d := newWalkState(ctx, opt)

	// apply sends entries out. If ctx is cancelled, short-circuit so we
	// don't block on a channel nobody is reading.
	d.apply = func(e *Entry) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- *e:
			return nil
		}
	}

	go func() {
		d.doWalk(names)
		d.dirWg.Wait()
		close(d.ch)
		d.wg.Wait()
		close(out)
	}()

	return out
}

// WalkFunc traverses the entries in 'names' in a concurrent fashion and calls
// 'apply' for entries that match criteria in 'opt'. The apply function must be
// concurrency-safe: it will be called concurrently from multiple go-routines.
//
// apply is also invoked for traversal error-entries (Err != nil); callbacks
// MUST check e.Err before touching e.Info. If apply returns a non-nil error
// the walk short-circuits: no further entries are delivered, and WalkFunc
// returns that error. A nil return from the callback (even on an
// error-entry) means "keep going".
//
// The *Entry passed to apply is only valid for the duration of the call.
// If the callback needs to retain any of the entry's data across calls
// (e.g. to store in a map), it must copy the Entry value itself rather
// than capturing the pointer.
func WalkFunc(ctx context.Context, names []string, opt Options, apply func(e *Entry) error) error {
	if opt.Concurrency <= 0 {
		opt.Concurrency = runtime.NumCPU()
	}

	// Derive a cancellable context so that an apply() error halts
	// remaining workers without requiring the caller's ctx to be cancelled.
	wctx, cancel := context.WithCancel(ctx)
	defer cancel()

	d := newWalkState(wctx, opt)

	// First non-nil callback error wins; subsequent errors are dropped.
	var (
		cbMu  sync.Mutex
		cbErr error
	)

	d.apply = func(e *Entry) error {
		if err := wctx.Err(); err != nil {
			return err
		}
		if err := apply(e); err != nil {
			cbMu.Lock()
			if cbErr == nil {
				cbErr = err
			}
			cbMu.Unlock()
			cancel()
			return err
		}
		return nil
	}

	d.doWalk(names)
	d.dirWg.Wait()
	close(d.ch)
	d.wg.Wait()

	cbMu.Lock()
	err := cbErr
	cbMu.Unlock()
	if err != nil {
		return err
	}
	// If the caller's ctx (not just our internal cancel) was done, surface it.
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return nil
}

func newWalkState(ctx context.Context, opt Options) *walkState {
	d := &walkState{
		Options: opt,
		ctx:     ctx,
		ch:      make(chan walkItem, opt.Concurrency),

		filterName: func(_ string) bool {
			return false
		},
		singlefs: func(_ *Entry) bool {
			return true
		},
	}

	if len(d.Excludes) > 0 {
		d.filterName = d.exclude
	}

	if d.OneFS {
		d.singlefs = d.isSingleFS
	}

	// default accept filter
	if d.Filter == nil {
		// by default - "don't filter anything"
		d.Filter = func(_ *Entry) (bool, error) {
			return false, nil
		}
	}

	// build a fast lookup of our types to stdlib; we will use
	// this in the output path (walkState.output)
	t := d.Type
	for k, v := range typMap {
		if (t & k) > 0 {
			d.typ |= v
		}
	}

	// create workers
	d.wg.Add(d.Concurrency)
	for i := 0; i < d.Concurrency; i++ {
		go d.worker()
	}
	return d
}

// walk the entries in 'names'; this creates workers to
// traverse the FS in a concurrent fashion.
func (d *walkState) doWalk(names []string) {
	dirs := make([]walkItem, 0, len(names))
	for i := range names {
		// observe cancellation promptly between roots
		if d.ctx.Err() != nil {
			return
		}

		nm := strings.TrimSuffix(names[i], "/")
		if len(nm) == 0 {
			nm = "/"
		}

		if d.filterName(nm) {
			continue
		}

		e := d.newEntry(0)
		if err := fio.Lstatm(nm, &e.Info); err != nil {
			d.sendErr(0, "lstat", nm, err)
			continue
		}

		// don't process entries we've already seen
		if d.isEntrySeen(e) {
			continue
		}

		skip, err := d.Filter(e)
		if err != nil {
			d.sendErr(0, "filter", nm, err)
			continue
		}
		if skip {
			continue
		}

		m := e.Mode()
		switch {
		case m.IsDir():
			if d.OneFS {
				d.trackFS(e)
			}
			dirs = append(dirs, walkItem{nm, 0})

		case (m & os.ModeSymlink) > 0:
			// we may have new info now. The symlink may point to file, dir or
			// special.
			dirs = d.doSymlink(e, dirs)

		default:
			d.output(e)
		}
	}

	d.enq(dirs)
}

// worker thread to walk directories
func (d *walkState) worker() {
	defer d.wg.Done()
	for it := range d.ch {
		// honor cancellation before touching the filesystem
		if d.ctx.Err() != nil {
			d.dirWg.Done()
			continue
		}

		e := d.newEntry(it.depth)
		if err := fio.Lstatm(it.path, &e.Info); err != nil {
			d.sendErr(it.depth, "lstat-wrk", it.path, err)
			d.dirWg.Done()
			continue
		}

		// we are _sure_ this is a dir.
		d.output(e)

		// Now process the contents of this dir
		d.walkPath(it.path, it.depth)

		// It is crucial that we do this as the last thing in the processing loop.
		// Otherwise, we have a race condition where the workers will prematurely quit.
		// We can only decrement this wait-group _after_ walkPath() has returned!
		d.dirWg.Done()
	}
}

// output action for entries we encounter
func (d *walkState) output(e *Entry) {
	m := e.Mode()

	// we have to special case regular files because there is
	// no mask for Regular Files!
	//
	// For everyone else, we can consult the typ map
	if (d.typ&m) > 0 || ((d.Type&FILE) > 0 && m.IsRegular()) {
		_ = d.apply(e)
	}
}

// sendErr builds a *walk.Error and emits it as an error-entry on the
// same channel as normal entries. Depth is the best-effort depth where
// the error was detected.
func (d *walkState) sendErr(depth int, op, name string, err error) {
	ee := &Entry{
		Depth: depth,
		Err:   &Error{Op: op, Name: name, Err: err},
	}
	_ = d.apply(ee)
}

// return true iff basename(nm) matches one of the patterns
func (d *walkState) exclude(nm string) bool {
	bn := path.Base(nm)
	for _, pat := range d.Excludes {
		ok, err := path.Match(pat, bn)
		if err != nil {
			d.sendErr(0, "exclude-glob", nm, fmt.Errorf("'%s': %w", pat, err))
		} else if ok {
			return true
		}
	}

	return false
}

// enqueue a list of dirs in a separate go-routine so the caller is
// not blocked (deadlocked).
//
// If ctx is cancelled mid-submit we must still release the dirWg tokens
// that were reserved at entry; otherwise dirWg.Wait() deadlocks.
func (d *walkState) enq(items []walkItem) {
	if len(items) == 0 {
		return
	}
	d.dirWg.Add(len(items))
	go func(items []walkItem) {
		for i, it := range items {
			select {
			case <-d.ctx.Done():
				// items[i..] were reserved but will never be submitted;
				// release their dirWg tokens so the walk can terminate.
				for j := i; j < len(items); j++ {
					d.dirWg.Done()
				}
				return
			case d.ch <- it:
			}
		}
	}(items)
}

// read a dir and return the names
func readDir(nm string) ([]string, error) {
	fd, err := os.Open(nm)
	if err != nil {
		return nil, &Error{"readdir", nm, err}
	}
	defer fd.Close()

	names, err := fd.Readdirnames(-1)
	if err != nil {
		return nil, &Error{"readdirnames", nm, err}
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
func (d *walkState) walkPath(nm string, depth int) {
	if d.ctx.Err() != nil {
		return
	}

	names, err := readDir(nm)
	if err != nil {
		// readDir wraps as *walk.Error; unwrap so sendErr can preserve Op/Name.
		if we, ok := err.(*Error); ok {
			d.sendErr(depth, we.Op, we.Name, we.Err)
		} else {
			d.sendErr(depth, "readdir", nm, err)
		}
		return
	}

	// hack to make joined paths not look like '//file'
	if nm == "/" {
		nm = ""
	}

	childDepth := depth + 1
	dirs := make([]walkItem, 0, len(names)/2)
	for i := range names {
		if d.ctx.Err() != nil {
			return
		}

		entry := names[i]

		// we don't want to use filepath.Join() because it "cleans"
		// the path (removes the leading .)
		fp := fmt.Sprintf("%s/%s", nm, entry)

		if d.filterName(fp) {
			continue
		}

		e := d.newEntry(childDepth)
		if err := fio.Lstatm(fp, &e.Info); err != nil {
			d.sendErr(childDepth, "lstat", fp, err)
			continue
		}

		// don't process entries we've already seen
		if d.isEntrySeen(e) {
			continue
		}

		skip, err := d.Filter(e)
		if err != nil {
			d.sendErr(childDepth, "filter", fp, err)
			continue
		}
		if skip {
			continue
		}

		m := e.Mode()
		switch {
		case m.IsDir():
			// don't descend if this directory is not on the same file system.
			if d.singlefs(e) {
				dirs = append(dirs, walkItem{fp, childDepth})
			}

		case (m & os.ModeSymlink) > 0:
			// we may have new info now. The symlink may point to file, dir or
			// special.
			dirs = d.doSymlink(e, dirs)

		default:
			d.output(e)
		}
	}

	d.enq(dirs)
}

// Walk symlinks and don't process dirs/entries that we've already seen
// This function updates dirs if the resolved symlink is a dir we have
// to descend - and returns the possibly updated dirs list. The resolved
// entry retains the symlink's Depth (descent does not restart depth
// counting).
func (d *walkState) doSymlink(e *Entry, dirs []walkItem) []walkItem {
	if !d.FollowSymlinks {
		d.output(e)
		return dirs
	}

	// process symlinks until we are done
	nm := e.Path()
	newnm, err := filepath.EvalSymlinks(nm)
	if err != nil {
		d.sendErr(e.Depth, "symlink", nm, err)
		return dirs
	}
	nm = newnm

	// we know this is no longer a symlink. Statm rewrites e.Info only;
	// e.Depth (the link's own depth) is untouched and is the depth we
	// report for the resolved target.
	if err = fio.Statm(nm, &e.Info); err != nil {
		d.sendErr(e.Depth, "symlink-stat", nm, err)
		return dirs
	}

	// do rest of processing iff we haven't seen this entry before.
	if !d.isEntrySeen(e) {
		switch {
		case e.Mode().IsDir():
			// Check if we crossed mountpoints after symlink
			// resolution.
			if d.singlefs(e) {
				dirs = append(dirs, walkItem{nm, e.Depth})
			}
		default:
			d.output(e)
		}
	}

	return dirs
}

// track this inode to detect loops; return true if we've seen it before
// false otherwise.
func (d *walkState) isEntrySeen(e *Entry) bool {
	if !d.IgnoreDuplicateInode {
		return false
	}

	key := fmt.Sprintf("%d:%d:%d", e.Dev, e.Rdev, e.Ino)
	_, loaded := d.ino.LoadOrStore(key, &e.Info)
	return loaded
}

// track this file for future mount points
// We call this function once for each entry passed to Walk().
func (d *walkState) trackFS(e *Entry) {
	key := fmt.Sprintf("%d:%d", e.Dev, e.Rdev)
	d.fs.Store(key, &e.Info)
}

// Return true if the inode is on the same file system as the command line args
func (d *walkState) isSingleFS(e *Entry) bool {
	key := fmt.Sprintf("%d:%d", e.Dev, e.Rdev)
	if _, ok := d.fs.Load(key); ok {
		return true
	}
	return false
}

// TODO mem pool for entry
func (d *walkState) newEntry(depth int) *Entry {
	return &Entry{Depth: depth}
}

// EOF
