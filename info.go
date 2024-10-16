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
	"fmt"
	"io/fs"
	"syscall"
	"time"
)

type Info struct {
	Nam   string
	Ino   uint64
	Nlink uint64

	Mod fs.FileMode
	Uid uint32
	Gid uint32
	_p0 uint32 // alignment pad

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
	var ii Info
	if err := Statm(nm, &ii); err != nil {
		return nil, err
	}
	return &ii, nil
}

func makeInfo(fi *Info, nm string, st *syscall.Stat_t, x Xattr) {
	*fi = Info{
		Nam:   nm,
		Ino:   st.Ino,
		Nlink: st.Nlink,
		Mod:   fs.FileMode(st.Mode & 0777),
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

	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fi.Mod |= fs.ModeDevice
	case syscall.S_IFCHR:
		fi.Mod |= fs.ModeDevice | fs.ModeCharDevice
	case syscall.S_IFDIR:
		fi.Mod |= fs.ModeDir
	case syscall.S_IFIFO:
		fi.Mod |= fs.ModeNamedPipe
	case syscall.S_IFLNK:
		fi.Mod |= fs.ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fi.Mod |= fs.ModeSocket
	}
	if st.Mode&syscall.S_ISGID != 0 {
		fi.Mod |= fs.ModeSetgid
	}
	if st.Mode&syscall.S_ISUID != 0 {
		fi.Mod |= fs.ModeSetuid
	}
	if st.Mode&syscall.S_ISVTX != 0 {
		fi.Mod |= fs.ModeSticky
	}
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

// Lstatm is like Lstat except it uses the caller's
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

func (ii *Info) String() string {
	return fmt.Sprintf("%s: %d; %s", ii.Name(), ii.Siz, ii.Mode().String())
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
