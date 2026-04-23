// hardlink_test.go -- tests for the hardlinker tracker
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
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
	"io/fs"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/opencoff/go-fio"
)

// makeRegularInfo builds a minimal *fio.Info that describes a regular
// file with a given link count. The mode bits are left at zero in the
// type portion so that Mode().IsRegular() returns true.
func makeRegularInfo(nlink uint32) *fio.Info {
	return &fio.Info{
		Ino:   42,
		Dev:   1,
		Rdev:  0,
		Mod:   0o644, // no type bits -> regular file
		Nlink: nlink,
	}
}

// TestHardlinkerConcurrent launches many goroutines that all call
// track() on the same source inode, each with a unique dst. Exactly
// one goroutine must see the "first caller" result (false); all the
// rest must see a hardlink result (true). This test exercises the
// atomicity fix for the Load/Store -> LoadOrStore change and is
// designed to be run under `go test -race`.
func TestHardlinkerConcurrent(t *testing.T) {
	const N = 1000

	h := newHardlinker()
	src := makeRegularInfo(2)

	var (
		falseCount atomic.Int64
		trueCount  atomic.Int64
		startWg    sync.WaitGroup
		doneWg     sync.WaitGroup
	)

	// Barrier: all workers park on startWg until we release them at
	// once, maximizing the window for a racy Load+Store to collide.
	startWg.Add(1)

	for i := 0; i < N; i++ {
		doneWg.Add(1)
		go func(id int) {
			defer doneWg.Done()
			dst := fmt.Sprintf("/dst/file-%d", id)
			startWg.Wait()
			if h.track(src, dst) {
				trueCount.Add(1)
			} else {
				falseCount.Add(1)
			}
		}(i)
	}

	// Release the barrier.
	startWg.Done()
	doneWg.Wait()

	if got := falseCount.Load(); got != 1 {
		t.Fatalf("expected exactly 1 'first caller' (false) result, got %d", got)
	}
	if got := trueCount.Load(); got != N-1 {
		t.Fatalf("expected %d hardlink (true) results, got %d", N-1, got)
	}

	// The primary map must contain exactly one entry for this inode.
	var mSize int
	h.m.Range(func(_, _ string) bool {
		mSize++
		return true
	})
	if mSize != 1 {
		t.Fatalf("expected 1 inode entry in h.m, got %d", mSize)
	}

	// The links map must contain N-1 entries, one per follower dst.
	var linksSize int
	h.links.Range(func(_, _ string) bool {
		linksSize++
		return true
	})
	if linksSize != N-1 {
		t.Fatalf("expected %d entries in h.links, got %d", N-1, linksSize)
	}
}

// TestHardlinkerSerialFirstCaller verifies the simple serial case: the
// very first track() for a given inode returns false (caller will copy);
// the second returns true (caller must hardlink).
func TestHardlinkerSerialFirstCaller(t *testing.T) {
	h := newHardlinker()
	src := makeRegularInfo(2)

	if linked := h.track(src, "/dst/a"); linked {
		t.Fatalf("first call should return false, got true")
	}
	if linked := h.track(src, "/dst/b"); !linked {
		t.Fatalf("second call should return true, got false")
	}

	// h.links should record b -> a.
	v, ok := h.links.Load("/dst/b")
	if !ok {
		t.Fatalf("expected /dst/b to be recorded in h.links")
	}
	if v != "/dst/a" {
		t.Fatalf("expected /dst/b -> /dst/a, got /dst/b -> %q", v)
	}
}

// TestHardlinkerSkipsNonRegular verifies that track() is a no-op (and
// returns false) for entries with Nlink == 1 or non-regular files; the
// internal maps must remain untouched.
func TestHardlinkerSkipsNonRegular(t *testing.T) {
	h := newHardlinker()

	// Case 1: regular file with Nlink == 1.
	single := makeRegularInfo(1)
	if linked := h.track(single, "/dst/single"); linked {
		t.Fatalf("Nlink==1 should return false, got true")
	}

	// Case 2: non-regular file (directory) with Nlink > 1.
	dir := &fio.Info{
		Ino:   99,
		Dev:   1,
		Mod:   fs.ModeDir | 0o755,
		Nlink: 3,
	}
	// Sanity: make sure the mode we picked actually reports non-regular.
	if dir.IsRegular() {
		t.Fatalf("test setup wrong: dir Info reports IsRegular()==true")
	}
	if linked := h.track(dir, "/dst/dir"); linked {
		t.Fatalf("non-regular file should return false, got true")
	}

	// Neither internal map should have any entries.
	var mSize, linksSize int
	h.m.Range(func(_, _ string) bool { mSize++; return true })
	h.links.Range(func(_, _ string) bool { linksSize++; return true })
	if mSize != 0 {
		t.Fatalf("expected h.m to be empty, got %d entries", mSize)
	}
	if linksSize != 0 {
		t.Fatalf("expected h.links to be empty, got %d entries", linksSize)
	}
}
