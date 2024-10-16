// marshal_test.go -- info marshal/unmarshal tests
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
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

func TestMarshal(t *testing.T) {
	assert := newAsserter(t)

	// make a fake info struct
	ii := randInfo()
	assert(ii != nil, "randinfo is nil")

	enc := make([]byte, 4096)
	z, err := ii.MarshalTo(enc)
	assert(err == nil, "marshal: err %s", err)
	assert(z == ii.MarshalSize(), "marshal: sz: exp %d, saw %d", ii.MarshalSize(), z)

	var di Info

	m, err := di.Unmarshal(enc[:z])
	assert(err == nil, "unmarshal: err %s", err)
	assert(m == z, "unmarshal: sz: exp %d, saw %d", z, m)

	err = infoEqual(ii, &di)
	assert(err == nil, "unmarshal: %s", err)
}

func TestMarshalMany(t *testing.T) {
	assert := newAsserter(t)
	n := rand.IntN(51200) + 1
	buf := make([]byte, 4096)

	for i := 0; i < n; i++ {
		ii := randInfo()
		want := ii.MarshalSize()
		assert(want < len(buf), "marshal: buf too small; have %d, want %d", len(buf), want)
		z, err := ii.MarshalTo(buf)
		assert(err == nil, "marshal: err %s", err)
		assert(z == ii.MarshalSize(), "marshal: sz: exp %d, saw %d", ii.MarshalSize(), z)

		var di Info

		m, err := di.Unmarshal(buf[:z])
		assert(err == nil, "unmarshal: err %s", err)
		assert(m == z, "unmarshal: sz: exp %d, saw %d", z, m)

		err = infoEqual(ii, &di)
		assert(err == nil, "unmarshal: %s", err)
	}
}

func TestMarshalErrors(t *testing.T) {
	assert := newAsserter(t)
	buf := make([]byte, 4096)

	ii := randInfo()
	z, err := ii.MarshalTo(buf[:128])
	assert(err != nil, "marshal: encoded to small buf: %d bytes", z)

	z, err = ii.MarshalTo(buf)
	assert(err == nil, "marshal: %s", err)
	assert(z == ii.MarshalSize(), "marshal: sz exp %d, saw %d", z, ii.MarshalSize())

	var di Info
	m, err := di.Unmarshal(buf[:z/2])
	assert(err != nil, "unmarshal: decoded small buf: %d bytes", m)
	assert(m == 0, "unmarshal: partial decode: %d", m)
}

func infoEqual(a, b *Info) error {
	if a.Nam != b.Nam {
		return fmt.Errorf("name: exp %s, saw %s", a.Nam, b.Nam)
	}
	if a.Ino != b.Ino {
		return fmt.Errorf("ino: exp %d, saw %d", a.Ino, b.Ino)
	}
	if a.Nlink != b.Nlink {
		return fmt.Errorf("nlink: exp %d, saw %d", a.Nlink, b.Nlink)
	}

	if a.Nlink != b.Nlink {
		return fmt.Errorf("nlink: exp %d, saw %d", a.Nlink, b.Nlink)
	}
	if a.Uid != b.Uid {
		return fmt.Errorf("uid: exp %d, saw %d", a.Uid, b.Uid)
	}
	if a.Gid != b.Gid {
		return fmt.Errorf("gid: exp %d, saw %d", a.Gid, b.Gid)
	}
	if a.Siz != b.Siz {
		return fmt.Errorf("size: exp %d, saw %d", a.Siz, b.Siz)
	}
	if a.Dev != b.Dev {
		return fmt.Errorf("dev: exp %d, saw %d", a.Dev, b.Dev)
	}
	if a.Rdev != b.Rdev {
		return fmt.Errorf("rdev: exp %d, saw %d", a.Rdev, b.Rdev)
	}

	if !a.Atim.Equal(b.Atim) {
		return fmt.Errorf("atime: exp %s, saw %s", a.Atim, b.Atim)
	}
	if !a.Mtim.Equal(b.Mtim) {
		return fmt.Errorf("mtime: exp %s, saw %s", a.Mtim, b.Mtim)
	}
	if !a.Ctim.Equal(b.Ctim) {
		return fmt.Errorf("ctime: exp %s, saw %s", a.Ctim, b.Ctim)
	}

	done := make(map[string]bool)
	for k, v := range a.Xattr {
		v2, ok := b.Xattr[k]
		if !ok {
			return fmt.Errorf("xattr: missing %s", k)
		}
		if v2 != v {
			return fmt.Errorf("xattr: %s: exp %s, saw %s", k, v, v2)
		}
		done[k] = true
	}

	for k := range b.Xattr {
		_, ok := done[k]
		if !ok {
			return fmt.Errorf("xattr: unknown key %s", k)
		}
	}
	return nil
}

func randInfo() *Info {
	ix := &Info{
		Nam:   randstr(32),
		Ino:   rand.Uint64() + 1,
		Nlink: rand.Uint64N(16) + 1,
		Uid:   rand.Uint32(),
		Gid:   rand.Uint32(),

		Siz:   rand.Int64() + 1,
		Dev:   rand.Uint64() + 1,
		Rdev:  rand.Uint64() + 1,
		Atim:  randtime(),
		Mtim:  randtime(),
		Ctim:  randtime(),
		Xattr: randxattr(rand.IntN(16) + 1),
	}

	if rand.Uint32()&1 > 0 {
		ix.Mod = uint32(fs.ModeDir)
	}

	ix.Mod |= 0600

	return ix
}

func randxattr(n int) Xattr {
	x := make(Xattr, n)

	for n > 0 {
		n -= 1
		kl := rand.IntN(32) + 1
		vl := rand.IntN(64) + 1
		k := randstr(kl)
		x[k] = randstr(vl)
	}
	return x
}

func randtime() time.Time {
	now := time.Now().UTC()
	dur := rand.Int64N(86400) + 1

	r := time.Duration(dur) * time.Second
	return now.Add(-r)
}

const ascii string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ/.0123456789"

func randstr(m int) string {
	const n = len(ascii)

	var w strings.Builder
	for m > 0 {
		m -= 1
		i := rand.IntN(n)
		w.WriteRune(rune(ascii[i]))
	}
	return w.String()
}
