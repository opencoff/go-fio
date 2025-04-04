// encdec.go  - handy wrappers for encoding/decoding basic types
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
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

func enc32[T ~int32 | ~uint32 | int](b []byte, n T) []byte {
	be := binary.BigEndian

	be.PutUint32(b, uint32(n))
	return b[4:]
}

func dec32[T ~int | ~int32 | ~uint | ~uint32](b []byte) ([]byte, T) {
	be := binary.BigEndian
	n := be.Uint32(b[:4])
	return b[4:], T(n)
}

func dec64[T ~int | ~int64 | ~uint | ~uint64](b []byte) ([]byte, T) {
	be := binary.BigEndian
	n := be.Uint64(b[:8])
	return b[8:], T(n)
}

func enc64[T ~int64 | ~uint64](b []byte, n T) []byte {
	be := binary.BigEndian
	be.PutUint64(b, uint64(n))
	return b[8:]
}

func encbytes(b []byte, s []byte) []byte {
	n := len(s)
	b = enc32(b, n)
	copy(b, s)
	return b[n:]
}

func decbytes(b []byte) (buf, out []byte, err error) {
	var n int
	if len(b) < 4 {
		return nil, nil, fmt.Errorf("unmarshal: bytes: buf len: %w", ErrTooSmall)
	}

	b, n = dec32[int](b)
	if n <= len(b) {
		return b[n:], b[:n], nil
	}

	return nil, nil, fmt.Errorf("unmarshal: bytes: buf: %w", ErrTooSmall)
}

func encstr(b []byte, s string) []byte {
	n := len(s)
	b = enc32(b, n)
	copy(b, []byte(s))
	return b[n:]
}

func decstr(b []byte) ([]byte, string, error) {
	if len(b) < 4 {
		return nil, "", fmt.Errorf("unmarshal: string len: %w", ErrTooSmall)
	}

	var n int
	b, n = dec32[int](b)
	if n <= len(b) {
		return b[n:], string(b[:n]), nil
	}
	return nil, "", fmt.Errorf("unmarshal: string: %w", ErrTooSmall)
}

// we represent time as a single uint64 in units of nanoseconds since
// the start of Unix time. This gives us reliable high precision time
// encoding for 584 years. This also means, we won't represent time values
// before Jan 1 1970.
func enctime(b []byte, t time.Time) []byte {
	ns := uint64(t.Unix()) * uint64(time.Second)
	ns += uint64(t.Nanosecond())
	return enc64(b, ns)
}

func dectime(b []byte) ([]byte, time.Time) {
	var val uint64
	b, val = dec64[uint64](b)

	ns := val % uint64(time.Second)
	s := val / uint64(time.Second)
	return b, time.Unix(int64(s), int64(ns))
}

var (
	ErrTooSmall = errors.New("buffer is not big enough")
)
