// info_marshal.go - marshal and unmarshal Info objects
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
	"path/filepath"
)

type MarshalFlag uint32

const (
	JunkPath MarshalFlag = 1 << iota

	// incrememnt this when we change our encoding format
	marshalVersion byte = 1
)

// MarshalSize returns the marshaled size of _this_
// instance of Info
func (ii *Info) MarshalSize(flag MarshalFlag) int {
	n := _FixedEncodingSize

	switch {
	case flag&JunkPath > 0:
		n += len(filepath.Base(ii.path)) + 4 // name + length

	default:
		n += len(ii.path) + 4 // name + length
	}
	n += ii.Xattr.MarshalSize()

	return 1 + n + 4
}

// MarshalTo marshals 'ii' into the provided buffer 'b'.
// The buffer 'b' is expected to be sufficiently big to hold the
// marshaled data. It returns the number of marshaled bytes
// (ie exactly the value returned by the corresponding MarshalSize()).
func (ii *Info) MarshalTo(b []byte, flag MarshalFlag) (int, error) {
	sz := ii.MarshalSize(flag)
	if len(b) < sz {
		return 0, fmt.Errorf("marshal: buf: %w", ErrTooSmall)
	}

	// let compiler know we are sized correctly
	_ = b[sz-1]

	// first set the length: the length we encode here is the
	// length of actual marshaled bytes.
	b = enc32(b, sz-4)

	b[0], b = marshalVersion, b[1:]
	b = enc64(b, ii.Ino)
	b = enc64(b, ii.Siz)
	b = enc64(b, ii.Dev)
	b = enc64(b, ii.Rdev)

	b = enc32(b, ii.Mod)
	b = enc32(b, ii.Uid)
	b = enc32(b, ii.Gid)
	b = enc32(b, ii.Nlink)

	b = enctime(b, ii.Atim)
	b = enctime(b, ii.Mtim)
	b = enctime(b, ii.Ctim)

	switch {
	case flag&JunkPath > 0:
		b = encstr(b, filepath.Base(ii.path))

	default:
		b = encstr(b, ii.path)
	}

	if _, err := ii.Xattr.MarshalTo(b); err != nil {
		return 0, err
	}
	return sz, nil
}

// Marshal marshals 'ii' into a correctly sized buffer and returns it
func (ii *Info) Marshal(flag MarshalFlag) ([]byte, error) {
	b := make([]byte, ii.MarshalSize(flag))
	_, err := ii.MarshalTo(b, flag)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// Unmarshal unmarshals the byte stream 'b' into a full rehydrated
// instance 'ii'. It returns the number of bytes consumed.
func (ii *Info) Unmarshal(b []byte) (int, error) {
	if len(b) < 4 {
		return 0, fmt.Errorf("unmarshal: len: %w", ErrTooSmall)
	}

	var z int

	b, z = dec32[int](b)
	if len(b) < z {
		return 0, fmt.Errorf("unmarshal: buf %d; want %d: %w", len(b), z, ErrTooSmall)
	}
	if z < _FixedEncodingSize {
		return 0, fmt.Errorf("unmarshal: buf exp %d, have %d: %w", z, len(b), ErrTooSmall)
	}

	// let compiler know we are sized correctly
	_ = b[z-1]

	var ver byte

	ver, b = b[0], b[1:]

	switch ver {
	case 1:
		return ii.unmarshalV1(b, z)
	}
	return 0, fmt.Errorf("unmarshal: unsupported version %d", ver)
}

func (ii *Info) unmarshalV1(b []byte, z int) (int, error) {
	b, ii.Ino = dec64[uint64](b)
	b, ii.Siz = dec64[int64](b)
	b, ii.Dev = dec64[uint64](b)
	b, ii.Rdev = dec64[uint64](b)

	b, mode := dec32[uint32](b)
	ii.Mod = fs.FileMode(mode)

	b, ii.Uid = dec32[uint32](b)
	b, ii.Gid = dec32[uint32](b)
	b, ii.Nlink = dec32[uint32](b)

	b, ii.Atim = dectime(b)
	b, ii.Mtim = dectime(b)
	b, ii.Ctim = dectime(b)

	var err error

	b, ii.path, err = decstr(b)
	if err != nil {
		return 0, err
	}

	ii.Xattr = make(Xattr)
	if _, err := ii.Xattr.Unmarshal(b); err != nil {
		return 0, err
	}
	return z + 4, nil
}
