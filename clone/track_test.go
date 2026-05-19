// track_test.go -- regression tests for the dircloner's dir-tracking.
//
// The cloner records which dst dirs had entries added or removed so
// that fixup() can restore their mtimes from the source after the
// copy/delete passes finish. doDel MUST record the parent directory
// of a deleted entry (not the grandparent); these tests pin that
// invariant and fail if an off-by-one Dir() creeps back in.
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

package clone

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencoff/go-fio/cmp"
	"github.com/puzpuzpuz/xsync/v4"
)

// newTrackTestCloner builds a minimal *dircloner with only the
// fields doDel / doCopy / doLink touch (dirs, Src, Dst).
func newTrackTestCloner() *dircloner {
	return &dircloner{
		Difference: &cmp.Difference{Src: "/src", Dst: "/dst"},
		dirs:       xsync.NewMap[string, bool](),
	}
}

// TestDoDelTracksParent asserts the invariant: deleting /.../a/b/c
// marks /.../a/b (the parent) as modified and does NOT mark /.../a
// (the grandparent) by itself.
func TestDoDelTracksParent(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "a", "b")
	grandparent := filepath.Join(root, "a")
	target := filepath.Join(parent, "c")

	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cc := newTrackTestCloner()
	if err := cc.doDel(target); err != nil {
		t.Fatalf("doDel: %v", err)
	}

	// Parent must be tracked.
	if _, ok := cc.dirs.Load(parent); !ok {
		t.Errorf("parent %q not tracked", parent)
	}

	// Grandparent must NOT be tracked by itself - an off-by-one
	// Dir() would have stored it instead of the parent.
	if _, ok := cc.dirs.Load(grandparent); ok {
		t.Errorf("grandparent %q tracked; indicates an off-by-one Dir() regression", grandparent)
	}
}

// TestDoCopyTracksParent pins the correct tracking behavior on the
// copy path so a future consolidation does not accidentally break
// what was already right.
func TestDoCopyTracksParent(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	src := filepath.Join(srcDir, "file")
	dst := filepath.Join(dstDir, "file")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	cc := newTrackTestCloner()
	if err := cc.doCopy(dst, src); err != nil {
		t.Fatalf("doCopy: %v", err)
	}

	// The dir that received the new file is dstDir.
	if _, ok := cc.dirs.Load(dstDir); !ok {
		t.Errorf("parent %q not tracked after doCopy", dstDir)
	}
	// root should NOT be tracked; only the immediate parent.
	if _, ok := cc.dirs.Load(root); ok {
		t.Errorf("grandparent %q tracked after doCopy (unexpected)", root)
	}
}

// TestDoLinkTracksParent mirrors TestDoCopyTracksParent for the
// hardlink path.
func TestDoLinkTracksParent(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	dstDir := filepath.Join(root, "dst")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dst: %v", err)
	}
	src := filepath.Join(srcDir, "orig")
	dst := filepath.Join(dstDir, "link")
	if err := os.WriteFile(src, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	cc := newTrackTestCloner()
	if err := cc.doLink(dst, src); err != nil {
		t.Fatalf("doLink: %v", err)
	}

	if _, ok := cc.dirs.Load(dstDir); !ok {
		t.Errorf("parent %q not tracked after doLink", dstDir)
	}
	if _, ok := cc.dirs.Load(root); ok {
		t.Errorf("grandparent %q tracked after doLink (unexpected)", root)
	}
}
