// proto_test.go -- round-trip and edge-case tests for the proto
// fio.Info marshaler.
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

package fio

import (
	"bytes"
	"io/fs"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

// TestProtoScalarRoundTrip exercises every scalar field an Info
// carries and asserts Marshal → Unmarshal preserves them exactly.
func TestProtoScalarRoundTrip(t *testing.T) {
	atim := time.Now().Truncate(time.Nanosecond)
	src := &Info{
		Ino:    0xDEADBEEF_CAFEBABE,
		Siz:    int64(1) << 40,
		Blocks: 8192,
		Dev:    0x1234_5678_9ABC_DEF0,
		Rdev:   0xFEDC_BA98_7654_3210,
		Mod:    fs.ModeDir | 0o755,
		Uid:    1000,
		Gid:    2000,
		Nlink:  42,
		Atim:   atim,
		Mtim:   atim.Add(time.Second),
		Ctim:   atim.Add(2 * time.Second),
		Xattr:  Xattr{},
	}
	src.SetPath("/some/deliberately/deep/path/file.ext")

	buf, err := src.Marshal(0)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var dst Info
	n, err := dst.Unmarshal(buf)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if n != len(buf) {
		t.Errorf("Unmarshal consumed %d bytes, want %d", n, len(buf))
	}

	if dst.Ino != src.Ino {
		t.Errorf("Ino: got %#x, want %#x", dst.Ino, src.Ino)
	}
	if dst.Siz != src.Siz {
		t.Errorf("Siz: got %d, want %d", dst.Siz, src.Siz)
	}
	if dst.Blocks != src.Blocks {
		t.Errorf("Blocks: got %d, want %d", dst.Blocks, src.Blocks)
	}
	if dst.Dev != src.Dev {
		t.Errorf("Dev: got %#x, want %#x", dst.Dev, src.Dev)
	}
	if dst.Rdev != src.Rdev {
		t.Errorf("Rdev: got %#x, want %#x", dst.Rdev, src.Rdev)
	}
	if dst.Uid != src.Uid {
		t.Errorf("Uid: got %d, want %d", dst.Uid, src.Uid)
	}
	if dst.Gid != src.Gid {
		t.Errorf("Gid: got %d, want %d", dst.Gid, src.Gid)
	}
	if dst.Nlink != src.Nlink {
		t.Errorf("Nlink: got %d, want %d", dst.Nlink, src.Nlink)
	}
	if dst.Mod != src.Mod {
		t.Errorf("Mod: got %v, want %v", dst.Mod, src.Mod)
	}
	if dst.Path() != src.Path() {
		t.Errorf("Path: got %q, want %q", dst.Path(), src.Path())
	}
}

// TestProtoTimeRoundTrip asserts signed-nanosecond timestamps
// round-trip for any era, including pre-1970. The proto schema
// uses int64 nanoseconds so negative values survive.
func TestProtoTimeRoundTrip(t *testing.T) {
	cases := []time.Time{
		time.Unix(0, 0),                              // epoch
		time.Now().Truncate(time.Nanosecond),         // now
		time.Date(1945, 8, 6, 8, 15, 0, 0, time.UTC), // pre-1970
		time.Date(1969, 12, 31, 23, 59, 59, 999_999_999, time.UTC),
		time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC), // far future
	}

	for _, want := range cases {
		src := &Info{
			Atim:  want,
			Mtim:  want.Add(time.Nanosecond),
			Ctim:  want.Add(-time.Nanosecond),
			Xattr: Xattr{},
		}
		src.SetPath("t")

		buf, err := src.Marshal(0)
		if err != nil {
			t.Fatalf("%v: Marshal: %v", want, err)
		}

		var dst Info
		if _, err := dst.Unmarshal(buf); err != nil {
			t.Fatalf("%v: Unmarshal: %v", want, err)
		}

		if !dst.Atim.Equal(src.Atim) {
			t.Errorf("Atim: got %v, want %v", dst.Atim, src.Atim)
		}
		if !dst.Mtim.Equal(src.Mtim) {
			t.Errorf("Mtim: got %v, want %v", dst.Mtim, src.Mtim)
		}
		if !dst.Ctim.Equal(src.Ctim) {
			t.Errorf("Ctim: got %v, want %v", dst.Ctim, src.Ctim)
		}
	}
}

// TestProtoXattrNULSafe is the concrete proof that Go strings are
// byte-safe (unlike C strings). An xattr value containing NUL bytes
// round-trips byte-for-byte through Marshal → Unmarshal via proto's
// `bytes` wire type.
func TestProtoXattrNULSafe(t *testing.T) {
	raw := []byte{0x02, 0x00, 0x00, 0x00, 0x01, 0x00, 0x06, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF, 0x04, 0x00, 0x04, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF, 0x10, 0x00, 0x05, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF, 0x20, 0x00, 0x07, 0x00,
		0xFF, 0xFF, 0xFF, 0xFF}

	src := &Info{
		Xattr: Xattr{
			"system.posix_acl_access": string(raw), // synthetic ACL-like blob with NULs
			"user.plain":              "hello",
			"user.bytes":              "a\x00b\x00c",
		},
	}
	src.SetPath("file")

	buf, err := src.Marshal(0)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var dst Info
	if _, err := dst.Unmarshal(buf); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if len(dst.Xattr) != len(src.Xattr) {
		t.Fatalf("xattr count: got %d, want %d", len(dst.Xattr), len(src.Xattr))
	}
	for k, wantVal := range src.Xattr {
		gotVal, ok := dst.Xattr[k]
		if !ok {
			t.Errorf("xattr %q missing after unmarshal", k)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("xattr %q: got %d bytes, want %d bytes; content mismatch",
				k, len(gotVal), len(wantVal))
		}
	}
}

// TestProtoJunkPathFlag verifies JunkPath strips the path to its
// basename on the wire but leaves the source Info unmodified after
// the call.
func TestProtoJunkPathFlag(t *testing.T) {
	const full = "/a/b/c/d/deliberately/deep/file.ext"
	const base = "file.ext"

	src := &Info{Xattr: Xattr{}}
	src.SetPath(full)

	// with JunkPath the unmarshaled path is the basename only
	buf, err := src.Marshal(JunkPath)
	if err != nil {
		t.Fatalf("Marshal JunkPath: %v", err)
	}

	var dst Info
	if _, err := dst.Unmarshal(buf); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dst.Path() != base {
		t.Errorf("JunkPath dst: got %q, want %q", dst.Path(), base)
	}

	// source Info.Path() is idempotent across the JunkPath call
	if src.Path() != full {
		t.Errorf("source Path mutated by JunkPath Marshal: got %q, want %q",
			src.Path(), full)
	}

	// without JunkPath the full path survives
	buf, err = src.Marshal(0)
	if err != nil {
		t.Fatalf("Marshal 0: %v", err)
	}
	var dst2 Info
	if _, err := dst2.Unmarshal(buf); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if dst2.Path() != full {
		t.Errorf("no-flag dst: got %q, want %q", dst2.Path(), full)
	}
}

// TestProtoDeterministicBytes verifies two successive Marshal calls
// on the same Info (with identical Xattr content but shuffled
// internal map iteration order) produce byte-for-byte identical
// output. This is the sort-by-key invariant that makes marshaled
// blobs usable as content-addressed keys.
func TestProtoDeterministicBytes(t *testing.T) {
	// Build two Info values with identical content but insert keys
	// into the map in different orders.
	keys := []string{"user.c", "user.a", "user.b", "user.z", "user.m"}
	val := "v"

	s1 := &Info{Xattr: Xattr{}}
	s1.SetPath("/f")
	for _, k := range keys {
		s1.Xattr[k] = val + k
	}

	s2 := &Info{Xattr: Xattr{}}
	s2.SetPath("/f")
	// reverse order
	for i := len(keys) - 1; i >= 0; i-- {
		s2.Xattr[keys[i]] = val + keys[i]
	}

	b1, err := s1.Marshal(0)
	if err != nil {
		t.Fatalf("Marshal s1: %v", err)
	}
	b2, err := s2.Marshal(0)
	if err != nil {
		t.Fatalf("Marshal s2: %v", err)
	}

	if !bytes.Equal(b1, b2) {
		t.Errorf("non-deterministic marshal: two Infos with identical content but\n"+
			"different map insertion order produced different bytes\n"+
			"  b1 len=%d\n  b2 len=%d", len(b1), len(b2))
	}
}

// TestProtoMarshalSizeMatchesMarshal verifies MarshalSize ==
// len(Marshal()) for a range of inputs including empty, small, and
// larger payloads.
func TestProtoMarshalSizeMatchesMarshal(t *testing.T) {
	cases := make([]*Info, 0, 3)

	// minimal
	minInfo := &Info{Xattr: Xattr{}}
	minInfo.SetPath("f")
	cases = append(cases, minInfo)

	// with xattrs of varying sizes
	now := time.Now()
	med := &Info{
		Atim:  now,
		Mtim:  now,
		Ctim:  now,
		Xattr: Xattr{"user.a": "1", "user.b": "hello world", "user.c": strings.Repeat("x", 1024)},
	}
	med.SetPath("/some/path")
	cases = append(cases, med)

	// junkpath set - copy the fields explicitly; Xattr is shared intentionally
	// since the marshal path does not mutate it.
	jp := *med
	cases = append(cases, &jp)

	for i, ii := range cases {
		flag := MarshalFlag(0)
		if i == 2 {
			flag = JunkPath
		}
		sz := ii.MarshalSize(flag)
		buf, err := ii.Marshal(flag)
		if err != nil {
			t.Fatalf("case %d: Marshal: %v", i, err)
		}
		if sz != len(buf) {
			t.Errorf("case %d: MarshalSize=%d, Marshal=%d bytes",
				i, sz, len(buf))
		}

		// MarshalTo into a fresh buffer of exactly that size
		b := make([]byte, sz)
		n, err := ii.MarshalTo(b, flag)
		if err != nil {
			t.Fatalf("case %d: MarshalTo: %v", i, err)
		}
		if n != sz {
			t.Errorf("case %d: MarshalTo returned %d, want %d", i, n, sz)
		}
	}
}

// TestProtoMarshalToShortBuffer surfaces ErrTooSmall for a caller
// buffer smaller than MarshalSize.
func TestProtoMarshalToShortBuffer(t *testing.T) {
	src := &Info{Xattr: Xattr{"user.a": "hello"}}
	src.SetPath("/some/path")

	sz := src.MarshalSize(0)
	tiny := make([]byte, sz/2)

	n, err := src.MarshalTo(tiny, 0)
	if err == nil {
		t.Fatalf("MarshalTo into short buffer: expected error, got nil (n=%d)", n)
	}
	if n != 0 {
		t.Errorf("MarshalTo returned n=%d on short-buffer error, want 0", n)
	}
}

// TestProtoMarshalMany stress-tests random Info values round-trip
// cleanly. Regression guard against field-ordering or
// boundary-condition bugs.
func TestProtoMarshalMany(t *testing.T) {
	for i := 0; i < 100; i++ {
		ii := randProtoInfo()
		buf, err := ii.Marshal(0)
		if err != nil {
			t.Fatalf("iter %d Marshal: %v", i, err)
		}
		var dst Info
		if _, err := dst.Unmarshal(buf); err != nil {
			t.Fatalf("iter %d Unmarshal: %v", i, err)
		}
		// path is the most visible field; full content parity is
		// asserted by TestProtoScalarRoundTrip.
		if dst.Path() != ii.Path() {
			t.Fatalf("iter %d: path mismatch: got %q, want %q",
				i, dst.Path(), ii.Path())
		}
	}
}

// randProtoInfo builds a randomized *Info for marshal round-trip
// fuzzing. math/rand/v2 is intentional: non-cryptographic RNG is the
// right tool for test fixtures.
//
//nolint:gosec // test fixture; non-cryptographic RNG is fine
func randProtoInfo() *Info {
	atim := time.Unix(rand.Int64N(4_000_000_000)-1_000_000_000, rand.Int64N(1_000_000_000))
	ii := &Info{
		Ino:    rand.Uint64(),
		Siz:    rand.Int64(),
		Blocks: rand.Int64N(1 << 32),
		Dev:    rand.Uint64(),
		Rdev:   rand.Uint64(),
		Uid:    rand.Uint32(),
		Gid:    rand.Uint32(),
		Nlink:  rand.Uint32N(16),
		Mod:    fs.FileMode(rand.Uint32N(0o777)),
		Atim:   atim,
		Mtim:   atim.Add(time.Duration(rand.Int64N(86400)) * time.Second),
		Ctim:   atim,
		Xattr:  make(Xattr, rand.IntN(4)),
	}
	ii.SetPath(randStr(rand.IntN(20) + 1))
	for i := 0; i < len(ii.Xattr); i++ {
		ii.Xattr["user."+randStr(4)] = randStr(rand.IntN(16) + 1)
	}
	return ii
}

const asciiLowerUpper = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

//nolint:gosec // test fixture; non-cryptographic RNG is fine
func randStr(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte(asciiLowerUpper[rand.IntN(len(asciiLowerUpper))])
	}
	return b.String()
}
