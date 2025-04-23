// xattr_marshal.go - marshal/unmarshal xattr
package fio

import (
	"fmt"
)

func (x *Xattr) MarshalSize() int {
	n := 4 // length of kv blob
	for k, v := range *x {
		n += (4 + 4) // lengths of each string
		n += len(k)
		n += len(v)
	}
	return n
}

func (x *Xattr) MarshalTo(b []byte) (int, error) {
	sz := x.MarshalSize()
	if len(b) < sz {
		return 0, fmt.Errorf("xattr marshal: %w", ErrTooSmall)
	}

	// let compiler know we are sized correctly
	_ = b[sz-1]

	// we'll write the blob len at the end
	blen, b := b[:4], b[4:]

	// first assemble the KV pairs
	for k, v := range *x {
		b = enc32(b, len(k))
		b = enc32(b, len(v))
		n := copy(b, []byte(k))
		b = b[n:]
		n = copy(b, []byte(v))
		b = b[n:]
	}

	// finally write the length of what we assembled
	enc32(blen, sz-4)
	return sz, nil
}

func (x *Xattr) Unmarshal(b []byte) (int, error) {
	if len(b) < 4 {
		return 0, fmt.Errorf("unmarshal: xattr: buf len %d: %w", len(b), ErrTooSmall)
	}

	var z int

	b, z = dec32[int](b)
	if len(b) < z {
		return 0, fmt.Errorf("unmarshal: xattr: buf len %d, want %d: %w", len(b), z, ErrTooSmall)
	}

	ret := z
	j := 0
	for z > 0 {
		var kl, vl int

		if len(b) < 8 {
			return 0, fmt.Errorf("unmarshal: xattr: %d: buf len %d, want 8: %w", j, len(b), ErrTooSmall)
		}

		b, kl = dec32[int](b)
		b, vl = dec32[int](b)
		z -= 8

		if len(b) < kl {
			return 0, fmt.Errorf("unmarshal: xattr: key %d: buf len %d, want %d: %w", j, len(b), kl, ErrTooSmall)
		}
		k := string(b[:kl])
		b = b[kl:]
		z -= kl

		if len(b) < vl {
			return 0, fmt.Errorf("unmarshal: xattr: key %d: buf len %d, want %d: %w", j, len(b), vl, ErrTooSmall)
		}
		v := string(b[:vl])
		b = b[vl:]
		z -= vl

		(*x)[k] = v
		j++
	}

	return ret + 4, nil
}
