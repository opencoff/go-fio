// info_test.go -- info tests
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

package fio

import (
	"errors"
	"os"
	"path"
	"syscall"
	"testing"
)

func TestBasicInfo(t *testing.T) {
	assert := newAsserter(t)

	tmp := t.TempDir()
	nm := path.Join(tmp, "testfile")
	err := mkfilex(nm)
	assert(err == nil, "test file %s: %s", nm, err)

	ii, err := Lstat(nm)
	assert(err == nil, "fio.Lstat: %s: %s", nm, err)

	fi, err := os.Lstat(nm)
	assert(err == nil, "os.Lstat: %s: %s", nm, err)

	assert(fi.Size() == ii.Size(), "size: exp %d, saw %d", fi.Size(), ii.Size())
	assert(fi.ModTime().Equal(ii.ModTime()), "mtime: exp %s, saw %s", fi.ModTime(), ii.ModTime())
	assert(fi.Mode() == ii.Mode(), "mode: exp %#b, saw %#b", fi.Mode(), ii.Mode())

	// Blocks must be populated to a sane non-zero value for any
	// regular file with content. Compare against syscall.Stat_t
	// directly to confirm the kernel-reported value made it
	// through makeInfo.
	var st syscall.Stat_t
	assert(syscall.Lstat(nm, &st) == nil, "syscall.Lstat: %s", nm)
	assert(ii.Blocks == st.Blocks,
		"blocks: exp %d, saw %d", st.Blocks, ii.Blocks)
	assert(ii.Blocks > 0, "blocks: zero for nonempty file %s", nm)

	// DiskBytes should round up to a filesystem block on any
	// content-bearing file: a 1KB-ish file uses at least one
	// 4KB filesystem block in practice.
	assert(ii.DiskBytes() == ii.Blocks*512,
		"DiskBytes mismatch: %d vs %d*512", ii.DiskBytes(), ii.Blocks)
}

func TestXattr(t *testing.T) {
	assert := newAsserter(t)

	tmp := t.TempDir()
	nm := path.Join(tmp, "testfile")
	err := mkfilex(nm)
	assert(err == nil, "test file %s: %s", nm, err)

	x, err := GetXattr(nm)
	assert(err == nil, "getxattr: %s", err)
	assert(x != nil, "xattr is nil?")

	x["user.foo.bar"] = nm

	err = SetXattr(nm, x)
	if err != nil && errors.Is(err, syscall.ENOTSUP) {
		t.Logf("no support for SetXattr on %s\n", tmp)
		return
	}
	assert(err == nil, "setxattr: %s", err)

	x, err = GetXattr(nm)
	assert(err == nil, "getxattr: %s", err)

	assert(x["user.foo.bar"] == nm, "xattr: user.foo.bar: %s", x["user.foo.bar"])
}
