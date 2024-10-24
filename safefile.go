// safefile.go - safe file creation and unwinding on error
//
// (c) 2021 Sudhi Herle <sudhi@herle.net>
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
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"sync/atomic"
)

// SafeFile is an io.WriteCloser which uses a temporary file that
// will be atomically renamed when there are no errors and
// caller invokes Close(). The recommended usage is:
//
//	sf, err := NewSafeFile(...)
//	... error handling
//
//	defer sf.Abort()
//
//	... write to sf ..
//	sf.Close()
//
// It is safe to call Abort on a closed SafeFile; the first call
// to Close() or Abort() seals the outcome. Similarly, it is safe
// to call Close() after Abort() - the first call to either
// takes precedence.
type SafeFile struct {
	*os.File

	// error for writes recorded once
	err  error
	name string // actual filename

	// tracks the state of this file:
	//  < 0 => aborted
	//  > 0 => closed
	//  = 0 => open and active
	closed atomic.Int64
}

var _ io.WriteCloser = &SafeFile{}

const (
	OPT_OVERWRITE uint32 = 1 << iota
	OPT_COW
)

// NewSafeFile creates a new temporary file that would either be
// aborted or safely renamed to the correct name.
// 'nm' is the name of the final file; if 'ovwrite' is true,
// then the file is overwritten if it exists.
func NewSafeFile(nm string, opts uint32, flag int, perm os.FileMode) (*SafeFile, error) {
	if st, err := Stat(nm); err == nil {
		if (opts & OPT_OVERWRITE) == 0 {
			return nil, fmt.Errorf("safefile: won't overwrite existing %s", nm)
		}

		if !st.Mode().IsRegular() {
			return nil, fmt.Errorf("safefile: %s is not a regular file", nm)
		}
	}

	// we need these two flags by default. The callers can set the rest..
	flag |= os.O_CREATE | os.O_TRUNC

	// make sure we don't have conflicting flags
	if (opts & OPT_COW) != 0 {
		flag &= ^os.O_WRONLY
		flag |= os.O_RDWR
	}

	if (flag & os.O_RDONLY) != 0 {
		return nil, fmt.Errorf("safefile: %s conflicting open mode (O_RDONLY)", nm)
	}

	if (flag & (os.O_RDWR | os.O_WRONLY)) == 0 {
		flag |= os.O_RDWR
	}

	// keep the old file around - we don't want to destroy it if we Abort() this operation.
	tmp := fmt.Sprintf("%s.tmp.%d.%x", nm, os.Getpid(), randU32())
	fd, err := os.OpenFile(tmp, flag, perm)
	if err != nil {
		return nil, err
	}

	// clone old file to the new one
	if (opts & OPT_COW) != 0 {
		old, err := os.Open(nm)
		switch {
		case err != nil:
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("safefile: open-cow: %w", err)
			}
		case err == nil:
			err = CopyFd(fd, old)
			old.Close()

			if err != nil {
				return nil, fmt.Errorf("safefile: %s: %w", nm, err)
			}
		}
	}

	sf := &SafeFile{
		File: fd,
		name: nm,
	}
	return sf, nil
}

func (sf *SafeFile) isOpen() bool {
	if n := sf.closed.Load(); n == 0 {
		return true
	}
	return false
}

var flag2str = []struct {
	flag int
	name string
}{
	{os.O_RDONLY, "rdonly"},
	{os.O_WRONLY, "wronly"},
	{os.O_RDWR, "rdwr"},
	{os.O_APPEND, "append"},
	{os.O_CREATE, "creat"},
	{os.O_EXCL, "excl"},
	{os.O_SYNC, "sync"},
	{os.O_TRUNC, "trunc"},
}

func prflag(flag int) string {
	var v []string

	for i := range flag2str {
		fl := &flag2str[i]
		if fl.flag&flag > 0 {
			v = append(v, fl.name)
		}
	}
	return strings.Join(v, ",")
}

// Attempt to write everything in 'b' and don't proceed if there was
// a previous error or the file was already closed.
func (sf *SafeFile) Write(b []byte) (int, error) {
	if sf.err != nil {
		return 0, sf.err
	}

	if !sf.isOpen() {
		return 0, fmt.Errorf("safefile: %s is not open", sf.Name())
	}

	var z int
	if z, sf.err = fullWrite(sf.File, b); sf.err != nil {
		return z, sf.err
	}
	return z, nil
}

// WriteAt writes 'b' at absolute offset 'off'
func (sf *SafeFile) WriteAt(b []byte, off int64) (int, error) {
	if sf.err != nil {
		return 0, sf.err
	}

	if !sf.isOpen() {
		return 0, fmt.Errorf("safefile: %s is not open", sf.Name())
	}
	n, err := sf.File.WriteAt(b, off)
	if err != nil {
		sf.err = err
	}
	return n, err
}

// Abort the file write and remove any temporary artifacts; it is safe
// to call Close() on a different code path; the first call to Abort() or
// Close() takes precedence.
func (sf *SafeFile) Abort() {
	n := sf.closed.Load()
	if n < 0 || n > 0 {
		return
	}

	sf.File.Close()
	os.Remove(sf.Name())
	sf.closed.Store(-1)

	// we retain any previous error in sf.err
}

// Close flushes all file data & metadata to disk, closes the file and atomically renames
// the temp file to the actual file - ONLY if there were no intervening errors.
func (sf *SafeFile) Close() error {
	if sf.err != nil {
		sf.Abort()
		return sf.err
	}

	n := sf.closed.Load()
	if n < 0 {
		if sf.err != nil {
			return sf.err
		}
		return errAborted
	}

	if n > 0 {
		return sf.err
	}

	if sf.err = sf.Sync(); sf.err != nil {
		return sf.err
	}

	if sf.err = sf.File.Close(); sf.err != nil {
		return sf.err
	}

	// mark this file as closed
	if sf.err = os.Rename(sf.Name(), sf.name); sf.err != nil {
		return sf.err
	}

	sf.closed.Store(1)

	return nil
}

func fullWrite(d *os.File, b []byte) (int, error) {
	var z int
	n := len(b)
	for n > 0 {
		m, err := d.Write(b)
		if err != nil {
			return z, fmt.Errorf("safefile: %w", err)
		}
		n -= m
		b = b[m:]
		z += m
	}
	return z, nil
}

func randU32() uint32 {
	var b [4]byte

	_, err := io.ReadFull(rand.Reader, b[:])
	if err != nil {
		panic(fmt.Sprintf("can't read 4 rand bytes: %s", err))
	}

	return binary.LittleEndian.Uint32(b[:])
}

func xflag2str(flag int) string {
	var v []string
	if flag&os.O_RDONLY > 0 {
		v = append(v, "rdonly")
	}
	if flag&os.O_WRONLY > 0 {
		v = append(v, "wronly")
	}
	if flag&os.O_RDWR > 0 {
		v = append(v, "rdwr")
	}
	if flag&os.O_APPEND > 0 {
		v = append(v, "append")
	}
	if flag&os.O_CREATE > 0 {
		v = append(v, "creat")
	}
	if flag&os.O_EXCL > 0 {
		v = append(v, "excl")
	}
	if flag&os.O_SYNC > 0 {
		v = append(v, "sync")
	}
	if flag&os.O_TRUNC > 0 {
		v = append(v, "trunc")
	}
	return strings.Join(v, ",")
}

var (
	errAborted = errors.New("safefile: aborted; file not committed")
)
