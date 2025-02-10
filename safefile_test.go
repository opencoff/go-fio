// safefile_test.go -- tests for safefile impl

package fio

import (
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	mrand "math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencoff/go-mmap"
)

func TestSafeFileSimple(t *testing.T) {
	assert := newAsserter(t)
	tmpdir := getTmpdir(t)

	fn := filepath.Join(tmpdir, "file-1")

	_, err := createFile(fn, 1024+mrand.IntN(65536))
	assert(err == nil, "can't create tmpfile: %s", err)

	sf, err := NewSafeFile(fn, 0, 0, 0600)
	assert(err != nil, "%s: bypassed overwrite protection", fn)

	buf := make([]byte, 128+mrand.IntN(65536))
	randbuf(buf)

	sf, err = NewSafeFile(fn, OPT_OVERWRITE, 0, 0600)
	assert(err == nil, "%s: can't create safefile: %s", fn, err)
	assert(sf != nil, "%s: nil ptr", fn)

	n, err := sf.Write(buf)
	assert(err == nil, "%s: write error: %s", sf.Name(), err)
	assert(n == len(buf), "%s: partial write: exp %d, saw %d", sf.Name(), len(buf), n)

	err = sf.Close()
	assert(err == nil, "%s: close: %s", sf.Name(), err)

	ck2 := cksum(buf)
	ck3, err := fileCksum(fn)
	assert(err == nil, "%s: cksum error: %s", fn, err)
	assert(byteEq(ck2, ck3), "cksum mismatch: %s\nexp %x\nsaw %x", fn, ck2, ck3)
}

func TestSafeFileAbort(t *testing.T) {
	assert := newAsserter(t)
	tmpdir := getTmpdir(t)

	fn := filepath.Join(tmpdir, "file-1")

	ck1, err := createFile(fn, 1024+mrand.IntN(65536))
	assert(err == nil, "can't create tmpfile: %s", err)

	buf := make([]byte, 128+mrand.IntN(65536))
	randbuf(buf)

	sf, err := NewSafeFile(fn, OPT_OVERWRITE, 0, 0600)
	assert(err == nil, "%s: can't create safefile: %s", fn, err)
	assert(sf != nil, "%s: nil ptr", fn)

	n, err := sf.Write(buf)
	assert(err == nil, "%s: write error: %s", sf.Name(), err)
	assert(n == len(buf), "%s: partial write: exp %d, saw %d", sf.Name(), len(buf), n)

	sf.Abort()
	err = sf.Close()
	assert(errors.Is(err, ErrAborted), "%s: abort+close: exp nil, saw %s", err)

	// File original contents shouldn't change
	ck3, err := fileCksum(fn)
	assert(err == nil, "%s: cksum error: %s", fn, err)
	assert(byteEq(ck1, ck3), "cksum mismatch: %s", fn)
}

func byteEq(a, b []byte) bool {
	return 1 == subtle.ConstantTimeCompare(a, b)
}

func cksum(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)[:]
}

func fileCksum(nm string) ([]byte, error) {
	fd, err := os.Open(nm)
	if err != nil {
		return nil, err
	}

	defer fd.Close()
	h := sha256.New()
	_, err = mmap.Reader(fd, func(b []byte) error {
		h.Write(b)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return h.Sum(nil)[:], nil
}

// create a file and return cryptographic checksum
func createFile(nm string, sz int) ([]byte, error) {
	fd, err := os.OpenFile(nm, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}

	defer fd.Close()

	if sz <= 0 {
		sz = 1024 + mrand.IntN(65536)
	}

	buf := make([]byte, 4096)
	h := sha256.New()

	// fill it with random data
	for sz > 0 {
		n := min(len(buf), sz)
		b := buf[:n]
		randbuf(b)
		h.Write(b)
		n, err := fd.Write(b)
		if err != nil {
			return nil, err
		}
		if n != len(b) {
			return nil, fmt.Errorf("%s: partial write (exp %d, saw %d)", nm, len(b), n)
		}
		sz -= n
	}

	if err = fd.Sync(); err != nil {
		return nil, err
	}

	if err = fd.Close(); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func randbuf(b []byte) []byte {
	n, err := crand.Read(b)
	if err != nil || n != len(b) {
		panic(fmt.Sprintf("can't read %d bytes of crypto/rand: %s", len(b), err))
	}
	return b
}
