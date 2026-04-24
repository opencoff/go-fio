// mknod_test.go -- tests for sysMode and mknod
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
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build linux || darwin

package clone

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/internal/testutil"
	"golang.org/x/sys/unix"
)

// TestSysMode verifies that sysMode translates the Go FileMode type
// bits into the matching POSIX S_IF* constants, preserving the low
// 9 permission bits.
func TestSysMode(t *testing.T) {
	assert := newAsserter(t)

	tests := []struct {
		name string
		in   fs.FileMode
		want uint32
	}{
		{
			name: "char-device-0644",
			in:   fs.ModeDevice | fs.ModeCharDevice | 0644,
			want: syscall.S_IFCHR | 0644,
		},
		{
			name: "block-device-0600",
			in:   fs.ModeDevice | 0600,
			want: syscall.S_IFBLK | 0600,
		},
		{
			name: "fifo-0666",
			in:   fs.ModeNamedPipe | 0666,
			want: syscall.S_IFIFO | 0666,
		},
		{
			name: "socket-0644",
			in:   fs.ModeSocket | 0644,
			want: syscall.S_IFSOCK | 0644,
		},
		{
			// No type bit set: sysMode should just yield the perm
			// bits unchanged (no S_IF* ORed in).
			name: "plain-0755",
			in:   0755,
			want: 0755,
		},
		{
			// A directory FileMode with no device/pipe/socket bits
			// set: sysMode doesn't understand S_IFDIR (Mknod isn't
			// used for dirs); should still return the perm bits.
			name: "dir-0755-no-type",
			in:   fs.ModeDir | 0755,
			want: 0755,
		},
	}

	for _, tc := range tests {
		got := sysMode(tc.in)
		assert(got == tc.want, "%s: exp %#o, saw %#o", tc.name, tc.want, got)
	}
}

// mknodStat performs a syscall.Stat on path and returns the result.
// Helper to keep the privileged tests readable.
func mknodStat(t *testing.T, path string) *syscall.Stat_t {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatalf("stat %s: %s", path, err)
	}
	return &st
}

// TestMknodCreatesCharDevice exercises the real mknod(2) path by
// re-creating /dev/null (major 1, minor 3). Needs CAP_MKNOD / root.
func TestMknodCreatesCharDevice(t *testing.T) {
	testutil.RequirePrivileged(t)
	assert := newAsserter(t)

	tmp := getTmpdir(t)
	dst := filepath.Join(tmp, "null")

	rdev := unix.Mkdev(1, 3)
	fi := &fio.Info{
		Mod:  fs.ModeDevice | fs.ModeCharDevice | 0644,
		Rdev: rdev,
	}
	fi.SetPath("synthetic-char")

	err := mknod(dst, fi)
	assert(err == nil, "mknod char: %s", err)
	t.Cleanup(func() { os.Remove(dst) })

	st := mknodStat(t, dst)
	assert(uint32(st.Mode)&syscall.S_IFMT == syscall.S_IFCHR,
		"char: mode %#o not S_IFCHR", st.Mode)
	assert(uint64(st.Rdev) == rdev,
		"char: rdev: exp %d, saw %d", rdev, st.Rdev)
}

// TestMknodCreatesBlockDevice mirrors the char-device test but asks
// for a block device (major 1, minor 0 — loop0-ish). Also root-only.
func TestMknodCreatesBlockDevice(t *testing.T) {
	testutil.RequirePrivileged(t)
	assert := newAsserter(t)

	tmp := getTmpdir(t)
	dst := filepath.Join(tmp, "blk")

	rdev := unix.Mkdev(1, 0)
	fi := &fio.Info{
		Mod:  fs.ModeDevice | 0600,
		Rdev: rdev,
	}
	fi.SetPath("synthetic-blk")

	err := mknod(dst, fi)
	assert(err == nil, "mknod blk: %s", err)
	t.Cleanup(func() { os.Remove(dst) })

	st := mknodStat(t, dst)
	assert(uint32(st.Mode)&syscall.S_IFMT == syscall.S_IFBLK,
		"blk: mode %#o not S_IFBLK", st.Mode)
	assert(uint64(st.Rdev) == rdev,
		"blk: rdev: exp %d, saw %d", rdev, st.Rdev)
}

// TestMknodCreatesFifo verifies mknod() for a FIFO. FIFOs don't
// require privilege — every user can mkfifo — so this is our
// always-on baseline that the sysMode + Mknod path works end-to-end.
func TestMknodCreatesFifo(t *testing.T) {
	assert := newAsserter(t)

	tmp := getTmpdir(t)
	dst := filepath.Join(tmp, "fifo")

	fi := &fio.Info{
		Mod:  fs.ModeNamedPipe | 0666,
		Rdev: 0,
	}
	fi.SetPath("synthetic-fifo")

	err := mknod(dst, fi)
	assert(err == nil, "mknod fifo: %s", err)
	t.Cleanup(func() { os.Remove(dst) })

	st := mknodStat(t, dst)
	assert(uint32(st.Mode)&syscall.S_IFMT == syscall.S_IFIFO,
		"fifo: mode %#o not S_IFIFO", st.Mode)
}
