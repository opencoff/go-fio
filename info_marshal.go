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
)

func (ii *Info) MarshalSize() int {
	n := len(ii.Nam)

	n += (2 * 8) + (4 * 4) + (3 * 8)
	n += (3 * 12) // 3 time fields

	n += xattrlen(ii.Xattr)
	return n + 4
}

func (ii *Info) MarshalTo(b []byte) (int, error) {
	sz := ii.MarshalSize()
	if len(b) < sz {
		return 0, fmt.Errorf("marshal: buf: %w", ErrTooSmall)
	}

	// let compiler know we are sized correctly
	_ = b[sz-1]

	// first set the length: the length we encode here is the
	// actual remaining marshaled bytes.
	b = enc32(b, sz-4)

	b = enc64(b, ii.Ino)
	b = enc64(b, ii.Nlink)

	b = enc32(b, ii.Mod)
	b = enc32(b, ii.Uid)
	b = enc32(b, ii.Gid)

	b = enc64(b, ii.Siz)
	b = enc64(b, ii.Dev)
	b = enc64(b, ii.Rdev)

	b = enctime(b, ii.Atim)
	b = enctime(b, ii.Mtim)
	b = enctime(b, ii.Ctim)

	b = encstr(b, ii.Nam)
	b = encxattr(b, ii.Xattr)
	return sz, nil
}

func (ii *Info) Marshal() ([]byte, error) {
	n := ii.MarshalSize() + 4
	b := make([]byte, n)
	if _, err := ii.MarshalTo(b); err != nil {
		return nil, err
	}

	return b, nil
}

func (ii *Info) Unmarshal(b []byte) (int, error) {
	if len(b) < 4 {
		return 0, fmt.Errorf("unmarshal: blob len: %w", ErrTooSmall)
	}

	var z int

	b, z = dec32[int](b)
	if len(b) < z {
		return 0, fmt.Errorf("unmarshal: buf %d: %w", z, ErrTooSmall)
	}

	b, ii.Ino = dec64[uint64](b)
	b, ii.Nlink = dec64[uint64](b)

	b, ii.Mod = dec32[uint32](b)
	b, ii.Uid = dec32[uint32](b)
	b, ii.Gid = dec32[uint32](b)

	b, ii.Siz = dec64[int64](b)
	b, ii.Dev = dec64[uint64](b)
	b, ii.Rdev = dec64[uint64](b)

	b, ii.Atim = dectime(b)
	b, ii.Mtim = dectime(b)
	b, ii.Ctim = dectime(b)

	var err error

	b, ii.Nam, err = decstr(b)
	if err != nil {
		return 0, err
	}

	b, ii.Xattr, err = decxattr(b)
	if err != nil {
		return 0, err
	}
	return z + 4, nil
}

func xattrlen(x Xattr) int {
	n := 4 // length of kv blob
	for k, v := range x {
		n += (4 + 4) // lengths of each string
		n += len(k)
		n += len(v)
	}
	return n
}

func encxattr(b []byte, x Xattr) []byte {
	z := 0

	// we'll write the blob len at the end
	blen := b[:4]
	b = b[4:]

	// first assemble the KV pairs
	for k, v := range x {
		b = enc32(b, len(k))
		b = enc32(b, len(v))
		z += 8
		n := copy(b, []byte(k))
		b = b[n:]
		z += n
		n = copy(b, []byte(v))
		b = b[n:]
		z += n
	}

	// finally write the length of what we assembled
	enc32(blen, z)
	return b
}

func decxattr(b []byte) ([]byte, Xattr, error) {
	if len(b) < 4 {
		return nil, nil, ErrTooSmall
	}

	var z int

	b, z = dec32[int](b)
	if len(b) < z {
		return nil, nil, ErrTooSmall
	}

	ret := b[z:]
	x := make(Xattr)
	for z > 0 {
		var kl, vl int

		if len(b) < 8 {
			return nil, nil, ErrTooSmall
		}

		b, kl = dec32[int](b)
		b, vl = dec32[int](b)
		z -= 8

		if len(b) < kl {
			return nil, nil, ErrTooSmall
		}
		k := string(b[:kl])
		b = b[kl:]
		z -= kl

		if len(b) < vl {
			return nil, nil, ErrTooSmall
		}
		v := string(b[:vl])
		b = b[vl:]
		z -= vl

		x[k] = v
	}

	return ret, x, nil
}
