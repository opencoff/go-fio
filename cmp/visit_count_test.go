// visit_count_test.go -- VisitSrc/VisitDst must fire exactly once
// per scanned entry. Earlier the engine fired them twice (once
// during the parallel scan walk, once again during the
// classification pass), which lied to progress observers.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026- Sudhi Herle <sudhi@herle.net>

package cmp_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/cmp"
)

// countingObserver records every VisitSrc/VisitDst call and the
// path the *fio.Info reports. Safe for concurrent use because cmp
// fires Visit* from many worker goroutines.
type countingObserver struct {
	src, dst atomic.Int64

	mu       sync.Mutex
	srcPaths map[string]int
	dstPaths map[string]int
}

func newCountingObserver() *countingObserver {
	return &countingObserver{
		srcPaths: make(map[string]int),
		dstPaths: make(map[string]int),
	}
}

func (c *countingObserver) VisitSrc(fi *fio.Info) {
	c.src.Add(1)
	c.mu.Lock()
	c.srcPaths[fi.Path()]++
	c.mu.Unlock()
}

func (c *countingObserver) VisitDst(fi *fio.Info) {
	c.dst.Add(1)
	c.mu.Lock()
	c.dstPaths[fi.Path()]++
	c.mu.Unlock()
}

// TestVisitOncePerEntry verifies that a synthetic tree of N src
// files (plus parent dirs) produces exactly N+dirs VisitSrc calls
// and 0 dst counts agree with the dst-side count. Critically, no
// path may show up more than once in either map.
func TestVisitOncePerEntry(t *testing.T) {
	assert := newAsserter(t)
	tmp := getTmpdir(t)

	src := filepath.Join(tmp, "lhs")
	dst := filepath.Join(tmp, "rhs")
	assert(os.MkdirAll(src, 0700) == nil, "mkdir src")
	assert(os.MkdirAll(dst, 0700) == nil, "mkdir dst")

	// Build a tree with a known shape on each side. Use distinct
	// shapes so we can tell the counters apart if they get crossed.
	srcFiles := []string{
		"a/f1", "a/f2", "a/f3",
		"b/c/g1", "b/c/g2",
		"top",
	}
	dstFiles := []string{
		"a/f1",     // common (will become CommonFiles)
		"a/f2",     // common
		"d/old1",   // right-only
		"d/old2",   // right-only
		"e/stale",  // right-only
	}
	for _, p := range srcFiles {
		assert(mkfilex(filepath.Join(src, p)) == nil, "mkfile src/%s", p)
	}
	for _, p := range dstFiles {
		assert(mkfilex(filepath.Join(dst, p)) == nil, "mkfile dst/%s", p)
	}

	// Count actual entries on each side via filepath.Walk so the
	// expected totals stay in sync with whatever mkfilex creates.
	wantSrc, wantDst := countEntries(t, src), countEntries(t, dst)

	obs := newCountingObserver()
	d, err := cmp.FsTree(context.Background(), src, dst, cmp.WithObserver(obs))
	assert(err == nil, "FsTree: %s", err)
	assert(d != nil, "nil Difference")

	gotSrc := obs.src.Load()
	gotDst := obs.dst.Load()

	assert(gotSrc == int64(wantSrc),
		"VisitSrc fired %d times, want %d (one per src entry)", gotSrc, wantSrc)
	assert(gotDst == int64(wantDst),
		"VisitDst fired %d times, want %d (one per dst entry)", gotDst, wantDst)

	// No path may have been visited more than once on either side.
	obs.mu.Lock()
	defer obs.mu.Unlock()
	for p, n := range obs.srcPaths {
		assert(n == 1, "src path %q visited %d times, want 1", p, n)
	}
	for p, n := range obs.dstPaths {
		assert(n == 1, "dst path %q visited %d times, want 1", p, n)
	}
}

// countEntries walks 'root' the boring way and returns the count of
// entries excluding root itself — matching what FsTree filters out
// via `if rel != "."`.
func countEntries(t *testing.T, root string) int {
	t.Helper()
	n := 0
	err := filepath.Walk(root, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatalf("countEntries(%s): %s", root, err)
	}
	return n
}
