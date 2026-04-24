// clonemode_symlink_test.go -- verifies clonemode does not chmod a
// symlink's target. os.Chmod follows symlinks, so a naive
// implementation would silently alter an unrelated file's mode when
// asked to restore mode on a cloned symlink.
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

//go:build unix

package clone

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/opencoff/go-fio"
)

// TestClonemodeDoesNotFollowSymlink plants a target file with a known
// mode, creates a symlink with a different (recorded) mode, invokes
// clonemode on the symlink, and asserts the target's mode is
// unchanged. A naive os.Chmod implementation would follow the symlink
// and clobber the target mode.
func TestClonemodeDoesNotFollowSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	symlink := filepath.Join(dir, "link")

	// target starts at 0600
	if err := os.WriteFile(target, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	// Manufacture an Info describing the symlink with a mode value
	// different from the target's 0600. clonemode picks the symlink
	// branch because fs.ModeSymlink is set.
	info := &fio.Info{
		Mod: fs.ModeSymlink | 0o777,
	}
	info.SetPath(symlink)

	if err := clonemode(symlink, info); err != nil {
		t.Fatalf("clonemode: %v", err)
	}

	// Target must still be 0600 - clonemode must NOT have followed
	// the symlink.
	st, err := os.Stat(target) // intentionally follows the symlink? no - this stats the real path
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("target mode changed: got %#o, want 0600 (clonemode followed the symlink)", got)
	}

	// Optional: on BSD/darwin lchmod should have updated the symlink's
	// own mode. On linux the helper is a no-op. Either way, the call
	// must not have errored (already asserted above).
	if runtime.GOOS != "linux" {
		// best-effort: Lstat the symlink to sanity-check it still
		// exists. Some kernels may or may not reflect the new mode.
		if _, err := os.Lstat(symlink); err != nil {
			t.Fatalf("lstat symlink after lchmod: %v", err)
		}
	}
}

// TestClonemodeChmodsNonSymlink confirms the non-symlink path still
// applies the mode normally.
func TestClonemodeChmodsNonSymlink(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "regular")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	info := &fio.Info{Mod: 0o640}
	info.SetPath(f)

	if err := clonemode(f, info); err != nil {
		t.Fatalf("clonemode: %v", err)
	}

	st, err := os.Stat(f)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o640 {
		t.Errorf("mode not applied: got %#o want 0640", got)
	}
}
