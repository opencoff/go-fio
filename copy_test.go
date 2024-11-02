// copy_test.go - file copy tests
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
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	assert := newAsserter(t)
	tmpdir := getTmpdir(t)

	src := filepath.Join(tmpdir, "file-a")
	dst := filepath.Join(tmpdir, "file-b")

	srcsum, err := createFile(src, 0)
	assert(err == nil, "create %s: %s", src, err)

	err = CopyFile(dst, src, 0600)
	assert(err == nil, "copy %s to %s: %s", src, dst, err)

	dstsum, err := fileCksum(dst)
	assert(err == nil, "cksum %s: %s", dst, err)
	assert(byteEq(srcsum, dstsum), "cksum mismatch: %s", dst)
}

var testDir = flag.String("testdir", "", "Use 'T' as the testdir for file I/O tests")

func getTmpdir(t *testing.T) string {
	assert := newAsserter(t)
	tmpdir := t.TempDir()

	if len(*testDir) > 0 {
		tmpdir = filepath.Join(*testDir, t.Name())
		err := os.MkdirAll(tmpdir, 0700)
		assert(err == nil, "mkdir %s: %s", tmpdir, err)
		t.Logf("Using %s as test dir .. \n", tmpdir)
		t.Cleanup(func() {
			t.Logf("cleaning up %s ..\n", tmpdir)
			os.RemoveAll(tmpdir)
		})
	}
	return tmpdir
}
