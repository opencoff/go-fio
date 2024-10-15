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
		Xattr: randxattr(rand.IntN(64) + 1),
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
