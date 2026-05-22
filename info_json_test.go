// info_json_test.go -- tests for Info.MarshalJSON
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
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"time"
)

// TestMarshalJSONFields exercises every field in the JSON output
// and pins the documented conversions (RFC3339Nano UTC timestamps,
// octal-string mod, base64 xattr values).
func TestMarshalJSONFields(t *testing.T) {
	atim := time.Date(2026, 5, 22, 10, 11, 12, 345000000, time.UTC)
	src := &Info{
		Ino:    0xCAFEBABE,
		Dev:    0x1234,
		Rdev:   0x5678,
		Siz:    1024,
		Mod:    fs.ModeDir | 0o755,
		Uid:    1000,
		Gid:    100,
		Nlink:  3,
		Atim:   atim,
		Mtim:   atim.Add(time.Second),
		Ctim:   atim.Add(2 * time.Second),
		Blocks: 8,
		Xattr: Xattr{
			"user.foo": "hello",
			// arbitrary octets including NUL and a high byte -
			// these are why xattr values are base64-encoded.
			"user.bar": "\x00\x01\x02\xff",
		},
	}
	src.SetPath("a/b/c.txt")

	buf, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if v := got["path"]; v != "a/b/c.txt" {
		t.Errorf("path: got %v, want a/b/c.txt", v)
	}
	// Perm() strips the ModeDir high bit, leaving 0755.
	if v := got["mod"]; v != "0755" {
		t.Errorf("mod: got %v, want 0755", v)
	}
	if v := got["siz"].(float64); v != 1024 {
		t.Errorf("siz: got %v, want 1024", v)
	}
	if v := got["blocks"].(float64); v != 8 {
		t.Errorf("blocks: got %v, want 8", v)
	}
	if v := got["ino"].(float64); v != float64(0xCAFEBABE) {
		t.Errorf("ino: got %v", v)
	}
	if v := got["uid"].(float64); v != 1000 {
		t.Errorf("uid: got %v", v)
	}
	if v := got["gid"].(float64); v != 100 {
		t.Errorf("gid: got %v", v)
	}
	if v := got["nlink"].(float64); v != 3 {
		t.Errorf("nlink: got %v", v)
	}

	if v := got["atim"]; v != "2026-05-22T10:11:12.345Z" {
		t.Errorf("atim: got %v", v)
	}
	if v := got["mtim"]; v != "2026-05-22T10:11:13.345Z" {
		t.Errorf("mtim: got %v", v)
	}
	if v := got["ctim"]; v != "2026-05-22T10:11:14.345Z" {
		t.Errorf("ctim: got %v", v)
	}

	xa, ok := got["xattr"].(map[string]any)
	if !ok {
		t.Fatalf("xattr missing or wrong type: %T", got["xattr"])
	}
	if v := xa["user.foo"]; v != "aGVsbG8=" {
		t.Errorf("xattr user.foo: got %v, want base64('hello')", v)
	}
	if v := xa["user.bar"]; v != "AAEC/w==" {
		t.Errorf("xattr user.bar: got %v, want base64('\\x00\\x01\\x02\\xff')", v)
	}
}

// TestMarshalJSONXattrOmittedWhenEmpty confirms the xattr field
// drops out entirely when an Info has no extended attributes
// (omitempty semantics).
func TestMarshalJSONXattrOmittedWhenEmpty(t *testing.T) {
	src := &Info{Mod: 0o644}
	src.SetPath("x")

	buf, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(buf), `"xattr"`) {
		t.Errorf("expected no xattr field, got: %s", buf)
	}
}

// TestMarshalJSONStableOrdering verifies that two Infos with the
// same xattr entries inserted in different orders produce
// byte-identical JSON. encoding/json sorts map keys lexically -
// this test pins that guarantee so a future change that drops
// it (e.g. moving to a slice) won't go unnoticed.
func TestMarshalJSONStableOrdering(t *testing.T) {
	a := &Info{Mod: 0o644, Xattr: Xattr{"z.last": "1", "a.first": "2", "m.mid": "3"}}
	b := &Info{Mod: 0o644, Xattr: Xattr{"a.first": "2", "m.mid": "3", "z.last": "1"}}
	a.SetPath("p")
	b.SetPath("p")

	ba, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal a: %v", err)
	}
	bb, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal b: %v", err)
	}
	if string(ba) != string(bb) {
		t.Errorf("xattr ordering not stable:\n  a=%s\n  b=%s", ba, bb)
	}
}

// TestMarshalJSONZeroValueTimestamps documents how the marshaler
// renders the zero time.Time - useful when callers construct an
// Info from a struct literal without populating timestamps.
func TestMarshalJSONZeroValueTimestamps(t *testing.T) {
	src := &Info{Mod: 0o644}
	src.SetPath("z")

	buf, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Zero time.Time renders as the Go epoch in UTC under RFC3339;
	// pin this so a future change to Format choice is caught.
	want := "0001-01-01T00:00:00Z"
	for _, k := range []string{"atim", "mtim", "ctim"} {
		if got[k] != want {
			t.Errorf("%s: got %v, want %s", k, got[k], want)
		}
	}
}

// TestMarshalJSONViaEncoder confirms the common consumer pattern
// (json.Encoder for NDJSON streaming) works end-to-end. Encode
// writes a trailing newline, which is exactly what NDJSON needs.
func TestMarshalJSONViaEncoder(t *testing.T) {
	src := &Info{Mod: 0o644, Siz: 7}
	src.SetPath("e")

	var sb strings.Builder
	if err := json.NewEncoder(&sb).Encode(src); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out := sb.String()
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("Encode output missing trailing newline: %q", out)
	}
	// One newline at the end, none in the middle.
	if n := strings.Count(out, "\n"); n != 1 {
		t.Errorf("Encode emitted %d newlines, want 1", n)
	}
}
