// info.go -- a better fs.FileInfo that also handles xattr
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
// warranty; it is provided "as is". No claim is made to its
// suitability for any purpose.

//go:generate ./scripts/gen-proto.sh proto/*.proto

package fio

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/opencoff/go-fio/internal/pb"
)

// Info represents a file/dir metadata in a normalized form.
// It satisfies the fs.FileInfo interface and notably supports
// extended file system attributes (`xattr(7)`). It can also be
// marshaled and unmarshaled into a portable byte stream.
//
// Callers construct an Info either by passing a *Info to Statm /
// Lstatm / Fstatm, or by struct literal (`&fio.Info{Mod: m}`).
// The wire encoding is handled via a local pb.Info inside the
// Marshal path; none of that surfaces to callers.
type Info struct {
	Ino  uint64
	Siz  int64
	Dev  uint64
	Rdev uint64

	// POSIX st_blocks: count of 512-byte allocation units. The
	// 512-byte unit is fixed by POSIX across every Unix kernel,
	// independent of the filesystem's actual block size. Use
	// DiskBytes() to recover bytes-on-disk.
	Blocks int64

	Mod   fs.FileMode
	Uid   uint32
	Gid   uint32
	Nlink uint32

	Atim time.Time
	Mtim time.Time
	Ctim time.Time

	path  string
	Xattr Xattr
}

var _ fs.FileInfo = &Info{}

// Stat is like os.Stat() but also returns xattr
func Stat(nm string) (*Info, error) {
	var ii Info
	if err := Statm(nm, &ii); err != nil {
		return nil, err
	}
	return &ii, nil
}

// Statm is like Stat above - except it uses caller
// supplied memory for the stat(2) info
func Statm(nm string, fi *Info) error {
	var st syscall.Stat_t

	if err := syscall.Stat(nm, &st); err != nil {
		return err
	}

	x, err := GetXattr(nm)
	if err != nil {
		return err
	}

	makeInfo(fi, nm, &st, x)
	return nil
}

// Lstat is like os.Lstat() but also returns xattr
func Lstat(nm string) (*Info, error) {
	var ii Info
	if err := Lstatm(nm, &ii); err != nil {
		return nil, err
	}
	return &ii, nil
}

// Lstatm is like Lstat except it uses the caller
// supplied memory.
func Lstatm(nm string, fi *Info) error {
	var st syscall.Stat_t
	if err := syscall.Lstat(nm, &st); err != nil {
		return err
	}

	x, err := LgetXattr(nm)
	if err != nil {
		return err
	}

	makeInfo(fi, nm, &st, x)
	return nil
}

// Fstat is like os.File.Stat() but also returns xattr
func Fstat(fd *os.File) (*Info, error) {
	var ii Info
	if err := Fstatm(fd, &ii); err != nil {
		return nil, err
	}
	return &ii, nil
}

// Fstatm is like Fstat except it uses caller supplied memory
func Fstatm(fd *os.File, fi *Info) error {
	return Lstatm(fd.Name(), fi)
}

// CopyTo does a deep-copy of the contents of ii to dest.
func (ii *Info) CopyTo(dest *Info) {
	old := dest.Xattr
	*dest = *ii
	if old == nil {
		old = make(Xattr)
	}

	// if there was an existing map in dest, we've saved it.
	// Else, we've created a new one. In either case, we
	// can now copy over the xattrs to this.
	for k, v := range ii.Xattr {
		old[k] = v
	}
	dest.Xattr = old
}

// Clone makes a deep copy of ii and returns the new
// instance
func (ii *Info) Clone() *Info {
	jj := new(Info)
	ii.CopyTo(jj)
	return jj
}

// String is a string representation of Info
func (ii *Info) String() string {
	return fmt.Sprintf("%s: %d %d; %s; %s", ii.Name(), ii.Siz, ii.Nlink, ii.ModTime().UTC(), ii.Mode().String())
}

// Path returns the relative path of this file ("relative" to current working dir
// of the calling process).
func (ii *Info) Path() string {
	return ii.path
}

// SetPath sets the path to 'p'
func (ii *Info) SetPath(p string) {
	ii.path = p
}

// fs.FileInfo methods of Info

// Name satisfies fs.FileInfo and returns the basename of the fs entry.
func (ii *Info) Name() string {
	return filepath.Base(ii.path)
}

// Size returns the fs entry's size
func (ii *Info) Size() int64 {
	return ii.Siz
}

// DiskBytes returns the actually-allocated bytes-on-disk for this
// entry. Differs from Size() for sparse files (where DiskBytes can
// be far smaller than Size) and for tiny files (where DiskBytes is
// rounded up to a filesystem block, so DiskBytes can exceed Size).
func (ii *Info) DiskBytes() int64 {
	return ii.Blocks * 512
}

// Mode returns the file mode bits
func (ii *Info) Mode() fs.FileMode {
	return ii.Mod
}

// ModTime returns the file modification time
func (ii *Info) ModTime() time.Time {
	return ii.Mtim
}

// IsDir returns true if this Info represents a directory entry
func (ii *Info) IsDir() bool {
	m := ii.Mode()
	return m.IsDir()
}

// IsRegular returns true if this Info represents a regular file
func (ii *Info) IsRegular() bool {
	m := ii.Mode()
	return m.IsRegular()
}

// IsSameFs returns true if a and b represent file entries on the
// same file system
func (a *Info) IsSameFS(b *Info) bool {
	if a.Dev == b.Dev && a.Rdev == b.Rdev {
		return true
	}
	return false
}

// Sys returns the platform specific info - in our case it
// returns a pointer to the underlying Info instance.
func (ii *Info) Sys() any {
	return ii
}

func ts2time(a syscall.Timespec) time.Time {
	t := time.Unix(a.Sec, a.Nsec)
	return t
}

// MarshalFlag tunes the marshaling of an Info.
type MarshalFlag uint32

const (
	// JunkPath strips Path() to its basename before marshaling.
	// Useful when the caller wants to serialize metadata without
	// leaking the full directory hierarchy.
	JunkPath MarshalFlag = 1 << iota
)

// MarshalSize returns the number of bytes Marshal / MarshalTo will
// write for this Info under the given flag.
func (ii *Info) MarshalSize(flag MarshalFlag) int {
	p := ii.toProto(flag)
	return p.SizeVT()
}

// Marshal allocates a correctly-sized buffer, marshals ii into it,
// and returns it.
func (ii *Info) Marshal(flag MarshalFlag) ([]byte, error) {
	p := ii.toProto(flag)
	return p.MarshalVT()
}

// MarshalTo marshals ii into the caller-supplied buffer, returning
// the number of bytes written. The buffer must be at least
// MarshalSize bytes.
func (ii *Info) MarshalTo(b []byte, flag MarshalFlag) (int, error) {
	p := ii.toProto(flag)
	sz := p.SizeVT()
	if len(b) < sz {
		return 0, fmt.Errorf("marshal: buf: %w", ErrTooSmall)
	}
	// MarshalToSizedBufferVT fills from the tail back to the head
	// of a buffer sized exactly to SizeVT; the final bytes land at
	// offset 0.
	if _, err := p.MarshalToSizedBufferVT(b[:sz]); err != nil {
		return 0, err
	}
	return sz, nil
}

// Unmarshal decodes a byte stream previously produced by Marshal /
// MarshalTo. Returns the number of bytes consumed.
func (ii *Info) Unmarshal(b []byte) (int, error) {
	var p pb.Info
	if err := p.UnmarshalVT(b); err != nil {
		return 0, err
	}
	ii.fromProto(&p)
	return len(b), nil
}

// toProto copies ii's fields into a local pb.Info. The pb.Info is
// a stack-allocatable wire-format carrier; we never let it leak
// into the public API.
//
// The xattr map is serialized into a sorted []*XattrEntry so
// identical Info values (regardless of map insertion order)
// produce byte-identical marshaled output. We build the entries
// slice directly from the map and sort it in place - no
// intermediate []string of keys, one pass of allocation.
func (ii *Info) toProto(flag MarshalFlag) *pb.Info {
	path := ii.path
	if flag&JunkPath != 0 {
		path = filepath.Base(path)
	}

	p := &pb.Info{
		Ino:    ii.Ino,
		Siz:    ii.Siz,
		Blocks: ii.Blocks,
		Dev:    ii.Dev,
		Rdev:   ii.Rdev,
		Mod:    uint32(ii.Mod),
		Uid:    ii.Uid,
		Gid:    ii.Gid,
		Nlink:  ii.Nlink,
		Atim:   ii.Atim.UnixNano(),
		Mtim:   ii.Mtim.UnixNano(),
		Ctim:   ii.Ctim.UnixNano(),
		Path:   path,
	}

	if n := len(ii.Xattr); n > 0 {
		p.Entries = make([]*pb.XattrEntry, 0, n)
		for k, v := range ii.Xattr {
			p.Entries = append(p.Entries, &pb.XattrEntry{
				Key:   k,
				Value: []byte(v),
			})
		}
		slices.SortFunc(p.Entries, func(a, b *pb.XattrEntry) int {
			return strings.Compare(a.Key, b.Key)
		})
	}

	return p
}

// fromProto copies a decoded pb.Info back into ii's Go-native fields.
func (ii *Info) fromProto(p *pb.Info) {
	ii.Ino = p.Ino
	ii.Siz = p.Siz
	ii.Blocks = p.Blocks
	ii.Dev = p.Dev
	ii.Rdev = p.Rdev
	ii.Mod = fs.FileMode(p.Mod)
	ii.Uid = p.Uid
	ii.Gid = p.Gid
	ii.Nlink = p.Nlink
	ii.Atim = time.Unix(0, p.Atim)
	ii.Mtim = time.Unix(0, p.Mtim)
	ii.Ctim = time.Unix(0, p.Ctim)
	ii.path = p.Path

	ii.Xattr = make(Xattr, len(p.Entries))
	for _, e := range p.Entries {
		ii.Xattr[e.Key] = string(e.Value)
	}
}
