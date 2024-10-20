// fileutils.go - utilities to make files and symlinks etc.

package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path"
	"time"

	"github.com/opencoff/go-mmap"
)

// make intermediate dirs as needed for 'dn'
func mkdir(dn string, tm time.Time) error {
	exists, err := DirExists(dn)
	if err != nil {
		return err
	}
	if !exists {
		if err = os.MkdirAll(dn, 0700); err != nil {
			return err
		}
	}
	return os.Chtimes(dn, tm, tm)
}

// make a random file of size 'size'; caller must ensure that
// the intermediate dirs are created.
func mkfile(fn string, size int64, tm time.Time) error {
	if err := mkdir(path.Dir(fn), tm); err != nil {
		return fmt.Errorf("mkdir %s: %w", path.Dir(fn), err)
	}

	fd, err := os.OpenFile(fn, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer fd.Close()

	const chunkSize int64 = 65536

	buf := make([]byte, chunkSize)
	for size > 0 {
		sz := min(size, chunkSize)
		randBytes(buf[:sz])
		n, err := fd.Write(buf[:sz])
		if err != nil {
			return err
		}
		size -= int64(n)
	}

	if err = fd.Sync(); err != nil {
		return err
	}

	if err = fd.Close(); err != nil {
		return err
	}

	return os.Chtimes(fn, tm, tm)
}

// mutate file nm and change between [minpct, maxpct) %
func mutate(fn string, minpct, maxpct int64) error {
	fd, err := os.OpenFile(fn, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer fd.Close()

	st, err := fd.Stat()
	if err != nil {
		return err
	}

	mm := mmap.New(fd)
	mapping, err := mm.Map(0, 0, mmap.PROT_WRITE|mmap.PROT_READ, 0)
	if err != nil {
		return err
	}

	sz := st.Size()
	n := mutateBytes(st.Size(), minpct, maxpct)
	buf := randBuf(n)
	ptr := mapping.Bytes()
	for i := 0; n > 0; i++ {
		off := rand.N(sz)
		ptr[off] = buf[i]
		n--
	}
	mapping.Unmap()

	// try to extend the file 30% of the time
	if rand.Float32() < 0.3 {
		if _, err := fd.Seek(0, 2); err != nil {
			return err
		}

		if _, err := fd.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func mutateBytes(sz int64, minp, maxp int64) int64 {
	x := (sz * minp) / 100
	y := (sz * maxp) / 100
	return rand.N(y-x) + x
}
