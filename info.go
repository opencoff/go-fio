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

// Info represents a file/dir metadata in a normalized form
// It satisfies the fs.FileInfo interface and notably supports
// extended file system attributes (`xattr(7)`). This type
// can also be safely marshaled and unmarshaled into a portable
// byte stream.
type Info struct {
	Ino  uint64
	Siz  int64
	Dev  uint64
	Rdev uint64

	Mod   fs.FileMode
	Uid   uint32
	Gid   uint32
	Nlink uint32

	Atim time.Time
	Mtim time.Time
	Ctim time.Time

	Nam   string
	Xattr Xattr
}

const (
	// The encoded size of the fixed-width elements of Info
	_FixedEncodingSize int = (3 * 12) + (4 * 4) + (4 * 8)
)

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

// String is a string representation of Info
func (ii *Info) String() string {
	return fmt.Sprintf("%s: %d; %s; %s", ii.Name(), ii.Siz, ii.ModTime(), ii.Mode().String())
}

// fs.FileInfo methods of Info

// Name satisfies fs.FileInfo and returns the name of the fs entry.
func (ii *Info) Name() string {
	return ii.Nam
}

// Size returns the fs entry's size
func (ii *Info) Size() int64 {
	return ii.Siz
}

// Mode returns the file mode bits
func (ii *Info) Mode() fs.FileMode {
	return fs.FileMode(ii.Mod)
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

// Sys returns the platform specific info - in our case it
// returns a pointer to the underlying Info instance.
func (ii *Info) Sys() any {
	return ii
}

func ts2time(a syscall.Timespec) time.Time {
	t := time.Unix(a.Sec, a.Nsec)
	return t
}
