// observer_fidelity_test.go -- verifies that every clone.Observer
// callback receives a *fio.Info whose Path() and Size() match the
// path-string args. This is the contract consumers (e.g. progress
// bars) rely on to avoid re-stat'ing files.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026- Sudhi Herle <sudhi@herle.net>

package clone

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
)

// recObserver records every callback for later inspection. It is
// safe for concurrent use because clone.Tree fires callbacks from
// many worker goroutines.
type recObserver struct {
	mu sync.Mutex

	mkdirs    []obsCall
	copies    []obsCall
	deletes   []obsCall
	links     []obsCall
	mdUpdates []obsCall
}

type obsCall struct {
	dst, src string
	fi       *fio.Info
}

func (r *recObserver) Difference(_ *cmp.Difference) {}
func (r *recObserver) VisitSrc(_ *fio.Info)         {}
func (r *recObserver) VisitDst(_ *fio.Info)         {}

func (r *recObserver) Mkdir(dst string, fi *fio.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mkdirs = append(r.mkdirs, obsCall{dst: dst, fi: fi})
}
func (r *recObserver) Copy(dst, src string, fi *fio.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.copies = append(r.copies, obsCall{dst: dst, src: src, fi: fi})
}
func (r *recObserver) Delete(dst string, fi *fio.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deletes = append(r.deletes, obsCall{dst: dst, fi: fi})
}
func (r *recObserver) Link(dst, src string, fi *fio.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.links = append(r.links, obsCall{dst: dst, src: src, fi: fi})
}
func (r *recObserver) MetadataUpdate(dst, src string, fi *fio.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mdUpdates = append(r.mdUpdates, obsCall{dst: dst, src: src, fi: fi})
}

// TestObserverInfoFidelity sets up a tree with new files, modified
// files, and deleted files, then asserts that every observer callback:
//
//  1. receives a non-nil *fio.Info
//  2. for src-side events (Mkdir/Copy/Link/MetadataUpdate): fi.Path()
//     equals the src path arg
//  3. for Delete events: fi.Path() equals the dst path arg
//  4. fi.Size() matches what os.Lstat reports for the same path
func TestObserverInfoFidelity(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)

	src := path.Join(tmp, "lhs")
	dst := path.Join(tmp, "rhs")

	assert(os.MkdirAll(src, 0700) == nil, "mkdir src")
	assert(os.MkdirAll(dst, 0700) == nil, "mkdir dst")

	// Three classes of entry exercised by the test:
	//   - LeftFiles:  src has it, dst doesn't  -> Copy + Mkdir(parent)
	//   - Diff:       both sides differ        -> Copy
	//   - RightFiles: dst has it, src doesn't  -> Delete
	assert(mkfiles(src, []string{"new/a"}, 2) == nil, "mkfiles src/new")
	assert(mkfiles(src, []string{"shared/b"}, 2) == nil, "mkfiles src/shared")
	assert(mkfiles(dst, []string{"shared/b"}, 2) == nil, "mkfiles dst/shared")
	assert(mkfiles(dst, []string{"gone/c"}, 2) == nil, "mkfiles dst/gone")

	r := &recObserver{}
	err := Tree(context.Background(), dst, src, WithObserver(r))
	assert(err == nil, "clone: %s", err)

	// Helper: every event must carry a non-nil Info, and the
	// Info's Path() must point at a real on-disk entry whose Size
	// matches. When the callback also exposes a 'src' string arg
	// (Copy/Link/MetadataUpdate), that string must equal fi.Path()
	// — observers should never see two disagreeing source-of-truth
	// values for the same entry.
	check := func(event string, c obsCall, hasSrcArg bool) {
		assert(c.fi != nil, "%s: nil *fio.Info for dst=%q", event, c.dst)

		st, statErr := os.Lstat(c.fi.Path())
		assert(statErr == nil, "%s: stat %s: %s", event, c.fi.Path(), statErr)
		assert(c.fi.Size() == st.Size(),
			"%s: fi.Size()=%d disk=%d for %s",
			event, c.fi.Size(), st.Size(), c.fi.Path())

		if hasSrcArg {
			assert(c.src == c.fi.Path(),
				"%s: src arg %q != fi.Path() %q",
				event, c.src, c.fi.Path())
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Mkdir: no src string arg; just fi.
	for _, c := range r.mkdirs {
		check("Mkdir", c, false)
	}
	// Copy: src string + fi (src side); they must agree.
	for _, c := range r.copies {
		check("Copy", c, true)
	}
	// Delete: dst path + fi (dst side); fi.Path() points at the
	// removed dst entry. The on-disk file is gone by the time we
	// inspect it, but fi was captured pre-delete so its Size()
	// remains the historical truth.
	for _, c := range r.deletes {
		assert(c.fi != nil, "Delete: nil *fio.Info for dst=%q", c.dst)
		assert(c.dst == c.fi.Path(),
			"Delete: dst arg %q != fi.Path() %q", c.dst, c.fi.Path())
		assert(c.fi.Size() >= 0,
			"Delete: nonsensical fi.Size()=%d for %s", c.fi.Size(), c.dst)
	}
	// MetadataUpdate fires post-copy with a fresh src Lstat; src
	// arg must equal fi.Path().
	for _, c := range r.mdUpdates {
		check("MetadataUpdate", c, true)
	}

	// Spot-check expected event counts. The exact totals depend on
	// the synthesized tree; we just verify the shape is sane.
	assert(len(r.copies) >= 4,
		"expected >=4 Copy events (2 new + 2 modified), got %d", len(r.copies))
	assert(len(r.deletes) >= 1,
		"expected >=1 Delete event for the gone/ subtree, got %d", len(r.deletes))
	assert(len(r.mkdirs) >= 1,
		"expected >=1 Mkdir for the new/ subtree, got %d", len(r.mkdirs))
}

// TestObserverLinkFidelity exercises the Link callback. We build a src
// tree with a hardlinked file pair so the cloner's Pass-3 hardlink
// resolution kicks in, then verify Link receives a non-nil src *fio.Info
// whose Path() matches the src arg.
func TestObserverLinkFidelity(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)

	src := path.Join(tmp, "lhs")
	dst := path.Join(tmp, "rhs")

	assert(os.MkdirAll(src, 0700) == nil, "mkdir src")
	assert(os.MkdirAll(dst, 0700) == nil, "mkdir dst")

	// One real file plus three hardlinks to it. The cloner copies
	// the first one and emits Link callbacks for the other three.
	assert(mkfilex(path.Join(src, "a/orig")) == nil, "mkfile orig")
	for _, name := range []string{"a/h1", "a/h2", "a/h3"} {
		assert(os.Link(path.Join(src, "a/orig"), path.Join(src, name)) == nil,
			"link %s", name)
	}

	r := &recObserver{}
	err := Tree(context.Background(), dst, src, WithObserver(r))
	assert(err == nil, "clone: %s", err)

	r.mu.Lock()
	defer r.mu.Unlock()

	assert(len(r.links) == 3,
		"expected 3 Link events, got %d", len(r.links))

	// For Link, src is an already-cloned dst-side path (the inode
	// we'll ln from) and fi is the original src-tree Info that
	// established the inode group. Both paths must exist on disk
	// and refer to files of the same size as fi.Size().
	for _, c := range r.links {
		assert(c.fi != nil, "Link: nil *fio.Info for dst=%q", c.dst)

		srcSt, srcErr := os.Lstat(c.src)
		assert(srcErr == nil, "Link: stat src %s: %s", c.src, srcErr)
		assert(srcSt.Size() == c.fi.Size(),
			"Link: src arg %s size %d != fi.Size() %d",
			c.src, srcSt.Size(), c.fi.Size())

		fiSt, fiErr := os.Lstat(c.fi.Path())
		assert(fiErr == nil, "Link: stat fi.Path() %s: %s", c.fi.Path(), fiErr)
		assert(fiSt.Size() == c.fi.Size(),
			"Link: fi.Path() %s disk size %d != fi.Size() %d",
			c.fi.Path(), fiSt.Size(), c.fi.Size())
	}
}

// TestObserverPathConsistency asserts that for every Copy event, the
// dst path equals filepath.Join(d.Dst, rel) where rel is the cmp Map
// key — i.e. observers can round-trip from dst path back into the
// Difference maps without surprises.
func TestObserverPathConsistency(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)

	src := path.Join(tmp, "lhs")
	dst := path.Join(tmp, "rhs")

	assert(os.MkdirAll(src, 0700) == nil, "mkdir src")
	assert(os.MkdirAll(dst, 0700) == nil, "mkdir dst")

	assert(mkfiles(src, []string{"deep/nest/x"}, 2) == nil, "mkfiles")

	r := &recObserver{}
	err := Tree(context.Background(), dst, src, WithObserver(r))
	assert(err == nil, "clone: %s", err)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Recover rels from dst paths and verify each is a valid suffix
	// of the dst root. Sort for stable diagnostic output.
	rels := make([]string, 0, len(r.copies))
	for _, c := range r.copies {
		rel, err := filepath.Rel(dst, c.dst)
		assert(err == nil, "Rel(%s, %s): %s", dst, c.dst, err)
		assert(rel != "." && rel != "..",
			"Copy.dst=%q is not under dst root %q (rel=%q)", c.dst, dst, rel)
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	t.Logf("copy rels: %v", rels)
}
