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
	"os"
	"path/filepath"
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

	path  string
	Xattr Xattr
}

const (
	// The encoded size of the fixed-width elements of Info
	// 1b for marhsal version
	// 8b for each time field x 3
	// 4b for each of uint32 x 3
	// 8b for each uint64 x 4
	_FixedEncodingSize int = 1 + (3 * 8) + (4 * 4) + (4 * 8)
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
