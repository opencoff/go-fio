// depth_test.go -- assert Entry.Depth is correct under a variety of
// walk configurations: plain walk, with Excludes, with Filter, with
// MaxDepth-like Filter, and with symlinks followed. The Depth reported
// on each emitted Entry must match the true filesystem depth relative
// to the root passed to Walk.

package walk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

// buildDepthFixture constructs a deterministic tree rooted at 'root':
//
//	root/               depth 0
//	├── a               depth 1
//	├── b/              depth 1
//	│   ├── c/          depth 2
//	│   │   └── d       depth 3
//	│   └── e           depth 2
//	├── skip/           depth 1
//	│   └── x           depth 2
//	└── sym -> b        depth 1 (symlink)
func buildDepthFixture(root string) error {
	mk := func(rel string) error { return mkfile(root, rel) }
	if err := mk("a"); err != nil {
		return err
	}
	if err := mk("b/c/d"); err != nil {
		return err
	}
	if err := mk("b/e"); err != nil {
		return err
	}
	if err := mk("skip/x"); err != nil {
		return err
	}

	// relative symlink "sym" -> "b" alongside it in root
	sym := filepath.Join(root, "sym")
	if err := os.Symlink("b", sym); err != nil {
		return err
	}
	return nil
}

// expectedDepths returns a map of absolute path → expected Depth for the
// fixture above, rooted at 'root'. Keys are every entry the walker is
// expected to emit when Type=ALL and no filters are applied.
func expectedDepths(root string) map[string]int {
	j := filepath.Join
	return map[string]int{
		root:              0,
		j(root, "a"):      1,
		j(root, "b"):      1,
		j(root, "b/c"):    2,
		j(root, "b/c/d"):  3,
		j(root, "b/e"):    2,
		j(root, "skip"):   1,
		j(root, "skip/x"): 2,
		j(root, "sym"):    1,
	}
}

// collect drains Walk and returns path→depth.
func collect(t *testing.T, roots []string, opt Options) map[string]int {
	t.Helper()
	got := make(map[string]int)

	och := Walk(context.Background(), roots, opt)
	var errs []error
	for e := range och {
		if e.Err != nil {
			errs = append(errs, e.Err)
			continue
		}
		got[e.Path()] = e.Depth
	}

	if len(errs) > 0 {
		t.Fatalf("unexpected walk errors: %v", errs)
	}
	return got
}

// assertDepths compares an observed path→depth map against an expected
// subset. Every path in 'expect' must be present in 'got' with the same
// depth; 'got' may contain additional entries (caller checks separately
// with assertExact if needed).
func assertDepths(t *testing.T, got, expect map[string]int) {
	t.Helper()
	for p, want := range expect {
		have, ok := got[p]
		if !ok {
			t.Errorf("missing path %q (expected depth=%d)", p, want)
			continue
		}
		if have != want {
			t.Errorf("path %q: depth=%d, want %d", p, have, want)
		}
	}
}

// assertExact asserts that got contains exactly the keys in expect, with
// matching depths. Extra keys fail the test.
func assertExact(t *testing.T, got, expect map[string]int) {
	t.Helper()
	assertDepths(t, got, expect)
	if len(got) != len(expect) {
		var extra []string
		for p := range got {
			if _, ok := expect[p]; !ok {
				extra = append(extra, fmt.Sprintf("%s(depth=%d)", p, got[p]))
			}
		}
		sort.Strings(extra)
		t.Errorf("got %d entries, want %d; unexpected: %s",
			len(got), len(expect), strings.Join(extra, ", "))
	}
}

// --- 1. Baseline: plain walk, Type=ALL --------------------------------------

func TestDepth_PlainWalk(t *testing.T) {
	root := t.TempDir()
	if err := buildDepthFixture(root); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	got := collect(t, []string{root}, Options{Type: ALL})
	assertExact(t, got, expectedDepths(root))
}

// --- 2. Multiple roots each count from 0 ------------------------------------

func TestDepth_MultipleRoots(t *testing.T) {
	base := t.TempDir()
	rootA := filepath.Join(base, "A")
	rootB := filepath.Join(base, "B")
	if err := os.MkdirAll(rootA, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(rootB, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := mkfile(rootA, "x/y"); err != nil {
		t.Fatal(err)
	}
	if err := mkfile(rootB, "p/q/r"); err != nil {
		t.Fatal(err)
	}

	got := collect(t, []string{rootA, rootB}, Options{Type: ALL})

	j := filepath.Join
	expect := map[string]int{
		rootA:             0,
		j(rootA, "x"):     1,
		j(rootA, "x/y"):   2,
		rootB:             0,
		j(rootB, "p"):     1,
		j(rootB, "p/q"):   2,
		j(rootB, "p/q/r"): 3,
	}
	assertExact(t, got, expect)
}

// --- 3. Excludes: removed subtree still leaves correct depth elsewhere ------

func TestDepth_WithExcludes(t *testing.T) {
	root := t.TempDir()
	if err := buildDepthFixture(root); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	got := collect(t, []string{root}, Options{
		Type:     ALL,
		Excludes: []string{"skip"},
	})

	j := filepath.Join
	expect := map[string]int{
		root:             0,
		j(root, "a"):     1,
		j(root, "b"):     1,
		j(root, "b/c"):   2,
		j(root, "b/c/d"): 3,
		j(root, "b/e"):   2,
		j(root, "sym"):   1,
	}
	assertExact(t, got, expect)
}

// --- 4. Filter: arbitrary skip, remaining depths must be correct ------------

func TestDepth_WithFilter(t *testing.T) {
	root := t.TempDir()
	if err := buildDepthFixture(root); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	// Filter drops the whole 'b' subtree (including the directory itself)
	// — so no 'b', 'b/c', 'b/c/d', 'b/e'. Everything else stays with
	// the right depth.
	got := collect(t, []string{root}, Options{
		Type: ALL,
		Filter: func(e *Entry) (bool, error) {
			return e.Name() == "b", nil
		},
	})

	j := filepath.Join
	expect := map[string]int{
		root:              0,
		j(root, "a"):      1,
		j(root, "skip"):   1,
		j(root, "skip/x"): 2,
		j(root, "sym"):    1,
	}
	assertExact(t, got, expect)
}

// --- 5. Filter observes correct Depth *inside* the Filter callback ----------
//
// This is the strongest guarantee: it proves depth is established before
// the Filter runs, so callers can implement -maxdepth-style pruning by
// returning true once Depth exceeds a bound.

func TestDepth_FilterSeesDepth(t *testing.T) {
	root := t.TempDir()
	if err := buildDepthFixture(root); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	// Record the depth the Filter sees for each path it inspects.
	var mu sync.Mutex
	seen := make(map[string]int)

	// MaxDepth semantic: skip anything deeper than 2.
	const maxDepth = 2
	got := collect(t, []string{root}, Options{
		Type: ALL,
		Filter: func(e *Entry) (bool, error) {
			mu.Lock()
			seen[e.Path()] = e.Depth
			mu.Unlock()
			return e.Depth > maxDepth, nil
		},
	})

	// Filter should have been consulted for every entry up to and including
	// the first hop past maxDepth on each branch (since Filter itself is
	// what prunes).
	expectSeen := map[string]int{
		root:                          0,
		filepath.Join(root, "a"):      1,
		filepath.Join(root, "b"):      1,
		filepath.Join(root, "b/c"):    2,
		filepath.Join(root, "b/c/d"):  3, // Filter is called, returns true → pruned
		filepath.Join(root, "b/e"):    2,
		filepath.Join(root, "skip"):   1,
		filepath.Join(root, "skip/x"): 2,
		filepath.Join(root, "sym"):    1,
	}
	assertDepths(t, seen, expectSeen)

	// Emitted set excludes pruned entries.
	expectGot := map[string]int{
		root:                          0,
		filepath.Join(root, "a"):      1,
		filepath.Join(root, "b"):      1,
		filepath.Join(root, "b/c"):    2,
		filepath.Join(root, "b/e"):    2,
		filepath.Join(root, "skip"):   1,
		filepath.Join(root, "skip/x"): 2,
		filepath.Join(root, "sym"):    1,
	}
	assertExact(t, got, expectGot)
}

// --- 6. Excludes + Filter combined — depth still right on survivors --------

func TestDepth_ExcludesAndFilter(t *testing.T) {
	root := t.TempDir()
	if err := buildDepthFixture(root); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	got := collect(t, []string{root}, Options{
		Type:     ALL,
		Excludes: []string{"skip"},
		Filter: func(e *Entry) (bool, error) {
			// drop the symlink itself (we're not following here)
			return e.Name() == "sym", nil
		},
	})

	j := filepath.Join
	expect := map[string]int{
		root:             0,
		j(root, "a"):     1,
		j(root, "b"):     1,
		j(root, "b/c"):   2,
		j(root, "b/c/d"): 3,
		j(root, "b/e"):   2,
	}
	assertExact(t, got, expect)
}

// --- 7. Symlink to directory: followed target inherits link's Depth ---------

func TestDepth_FollowSymlinkRetainsLinkDepth(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks unreliable on Windows tempdirs")
	}

	// Canonicalize the tempdir path: on macOS, t.TempDir() can return
	// `/tmp/...` while EvalSymlinks (used by the walker when following)
	// resolves to `/private/tmp/...`. Working in the canonical form keeps
	// every emitted path on the same prefix so assertions are stable.
	rawRoot := t.TempDir()
	root, err := filepath.EvalSymlinks(rawRoot)
	if err != nil {
		t.Fatalf("evalsymlinks root: %v", err)
	}

	// layout:
	//   root/
	//   ├── real/            depth 1
	//   │   └── inside       depth 2
	//   └── link -> real     depth 1 (symlink to dir)
	if err := mkfile(root, "real/inside"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	// With FollowSymlinks=true the walker resolves `link` to `real` and
	// descends. The emitted entry for what was reached via the link has
	// Path()=`{root}/real` (post-resolution) and Depth=1 (the link's own
	// depth — descent through the link does not restart depth counting).
	// IgnoreDuplicateInode is on so we don't double-yield 'real' and its
	// descendants: direct descent sees them first, the followed-link
	// resolution finds the inode already recorded and skips re-output.
	got := collect(t, []string{root}, Options{
		Type:                 ALL,
		FollowSymlinks:       true,
		IgnoreDuplicateInode: true,
	})

	j := filepath.Join
	expect := map[string]int{
		root:                   0,
		j(root, "real"):        1,
		j(root, "real/inside"): 2,
	}
	for p, want := range expect {
		have, ok := got[p]
		if !ok {
			t.Errorf("missing %q (want depth=%d); got paths: %v", p, want, keys(got))
			continue
		}
		if have != want {
			t.Errorf("%q: depth=%d, want %d", p, have, want)
		}
	}
}

func keys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// --- 8. WalkFunc path sees the same depths as Walk -------------------------
//
// The pointer-based apply callback must observe identical depths to the
// value-channel form; we assert equivalence across both APIs.

func TestDepth_WalkFuncMatchesWalk(t *testing.T) {
	root := t.TempDir()
	if err := buildDepthFixture(root); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	viaChan := collect(t, []string{root}, Options{Type: ALL})

	viaFunc := make(map[string]int)
	var mu sync.Mutex
	err := WalkFunc(context.Background(), []string{root}, Options{Type: ALL}, func(e *Entry) error {
		if e.Err != nil {
			return e.Err
		}
		mu.Lock()
		viaFunc[e.Path()] = e.Depth
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("WalkFunc: %v", err)
	}

	assertExact(t, viaFunc, viaChan)
}
