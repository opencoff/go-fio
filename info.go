// info.go - a better fs.FileInfo that also handles xattr
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
	"io/fs"

	"syscall"
	"time"
)

type Info struct {
	Nam   string
	Ino   uint64
	Nlink uint64
	Mod   uint32
	Uid   uint32
	Gid   uint32
	_p0   uint32 // alignment pad

	Siz  int64
	Dev  uint64
	Rdev uint64

	Atim time.Time
	Mtim time.Time
	Ctim time.Time

	Xattr Xattr
}

var _ fs.FileInfo = &Info{}

// Stat is like os.Stat() but also returns xattr
func Stat(nm string) (*Info, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(nm, &st); err != nil {
		return nil, err
	}

	x, err := GetXattr(nm)
	if err != nil {
		return nil, err
	}

	s := &Info{
		Nam:   nm,
		Ino:   st.Ino,
		Nlink: st.Nlink,
		Mod:   st.Mode,
		Uid:   st.Uid,
		Gid:   st.Gid,
		Siz:   st.Size,
		Dev:   st.Dev,
		Rdev:  st.Rdev,
		Atim:  ts2time(st.Atim),
		Mtim:  ts2time(st.Mtim),
		Ctim:  ts2time(st.Ctim),
		Xattr: x,
	}

	return s, nil
}

// Stat is like os.Lstat() but also returns xattr
func Lstat(nm string) (*Info, error) {
	var st syscall.Stat_t
	if err := syscall.Lstat(nm, &st); err != nil {
		return nil, err
	}

	x, err := LgetXattr(nm)
	if err != nil {
		return nil, err
	}

	s := &Info{
		Nam:   nm,
		Ino:   st.Ino,
		Nlink: st.Nlink,
		Mod:   st.Mode,
		Uid:   st.Uid,
		Gid:   st.Gid,
		Siz:   st.Size,
		Dev:   st.Dev,
		Rdev:  st.Rdev,
		Atim:  ts2time(st.Atim),
		Mtim:  ts2time(st.Mtim),
		Ctim:  ts2time(st.Ctim),
		Xattr: x,
	}

	return s, nil
}

// fs.FileInfo methods of Info
func (ii *Info) Name() string {
	return ii.Nam
}

func (ii *Info) Size() int64 {
	return ii.Siz
}

func (ii *Info) Mode() fs.FileMode {
	return fs.FileMode(ii.Mod)
}

func (ii *Info) ModTime() time.Time {
	return ii.Mtim
}

func (ii *Info) IsDir() bool {
	m := ii.Mode()
	return m.IsDir()
}

func (ii *Info) Sys() any {
	return ii
}

func ts2time(a syscall.Timespec) time.Time {
	return time.Unix(a.Sec, a.Nsec)
}
