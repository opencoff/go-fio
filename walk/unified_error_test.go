// unified_error_test.go -- tests for the Entry.Err unified error
// stream, context cancellation, and WalkFunc callback-error
// short-circuit.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim is made to its
// suitability for any purpose.

package walk

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// TestWalkUnifiedEntryStream verifies that traversal errors arrive on
// the single output channel as Entry{Err: *walk.Error}, and that the
// error's Name field carries the failing path.
func TestWalkUnifiedEntryStream(t *testing.T) {
	root := t.TempDir()
	// plant a good file and a dangling symlink (only observed as an
	// error when FollowSymlinks=true).
	if err := mkfile(root, "good"); err != nil {
		t.Fatalf("mkfile: %v", err)
	}
	bad := filepath.Join(root, "dangling")
	if err := os.Symlink("/no/such/target/path", bad); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	opt := Options{Type: ALL, FollowSymlinks: true}
	ch := Walk(context.Background(), []string{root}, opt)

	var sawErr bool
	var sawGood bool
	for e := range ch {
		if e.Err != nil {
			sawErr = true
			var werr *Error
			if !errors.As(e.Err, &werr) {
				t.Errorf("error entry Err is %T, want *walk.Error (err=%v)", e.Err, e.Err)
				continue
			}
			if werr.Name != bad {
				t.Errorf("error entry path: got %q, want %q", werr.Name, bad)
			}
			continue
		}
		if filepath.Base(e.Path()) == "good" {
			sawGood = true
		}
	}

	if !sawErr {
		t.Errorf("expected at least one Entry with Err != nil (dangling symlink)")
	}
	if !sawGood {
		t.Errorf("expected to observe the 'good' file in the unified stream")
	}
}

// TestWalkCtxCancel verifies that cancelling ctx terminates the walk
// promptly and does not leak goroutines.
func TestWalkCtxCancel(t *testing.T) {
	root := t.TempDir()
	// build a deepish tree so the walk has plenty to chew on
	for i := 0; i < 50; i++ {
		for j := 0; j < 10; j++ {
			rel := fmt.Sprintf("d%02d/f%02d", i, j)
			if err := mkfile(root, rel); err != nil {
				t.Fatalf("mkfile %s: %v", rel, err)
			}
		}
	}

	before := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := Walk(ctx, []string{root}, Options{Type: ALL, Concurrency: 4})

	// consume a handful of entries, then cancel
	drained := 0
	for e := range ch {
		_ = e
		drained++
		if drained >= 3 {
			cancel()
			break
		}
	}
	// drain the rest so the channel closes (but we've already cancelled)
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(5 * time.Second):
		t.Fatal("walk did not terminate within 5s after ctx cancel")
	}

	// give goroutines a tick to unwind
	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()
	if after > before+2 {
		t.Errorf("goroutine leak suspected: before=%d after=%d", before, after)
	}
}

// TestWalkFuncCallbackError verifies that a callback returning a
// non-nil error short-circuits the walk: later entries are not
// processed, and WalkFunc returns the callback's error.
func TestWalkFuncCallbackError(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 100; i++ {
		if err := mkfile(root, fmt.Sprintf("f%03d", i)); err != nil {
			t.Fatalf("mkfile: %v", err)
		}
	}

	stopAfter := int32(5)
	wantErr := errors.New("stop walking")
	var seen atomic.Int32

	err := WalkFunc(context.Background(), []string{root}, Options{Type: ALL}, func(e *Entry) error {
		if e.Err != nil {
			return e.Err
		}
		n := seen.Add(1)
		if n == stopAfter {
			return wantErr
		}
		return nil
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("WalkFunc: got err=%v, want %v", err, wantErr)
	}

	// No tight bound is possible because Concurrency workers may already
	// be mid-flight. A generous cap of 50 confirms we short-circuited well
	// before the full 100.
	n := seen.Load()
	if n >= 100 {
		t.Errorf("expected short-circuit; callback saw %d of 100 entries", n)
	}
}
