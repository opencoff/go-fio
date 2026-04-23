// copy_linux_test.go - linux-specific file copy tests
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

//go:build linux

package fio_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencoff/go-fio"
)

// TestCopyFdPreservesOffsets verifies that CopyFd does not disturb the
// caller's fd offsets on either source or destination. Both the
// FIO_CLONE (reflink) and copy_file_range(2) paths are documented to
// leave fd offsets untouched when explicit offset arguments are supplied.
func TestCopyFdPreservesOffsets(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src")
	dstPath := filepath.Join(dir, "dst")
	data := []byte("hello world, this is some content for copy")
	if err := os.WriteFile(srcPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dstPath, make([]byte, len(data)), 0644); err != nil {
		t.Fatal(err)
	}

	src, err := os.OpenFile(srcPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	dst, err := os.OpenFile(dstPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer dst.Close()

	// move both fds to deliberate non-zero offsets
	if _, err := src.Seek(7, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := dst.Seek(13, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	if err := fio.CopyFd(dst, src); err != nil {
		t.Fatal(err)
	}

	srcOff, _ := src.Seek(0, io.SeekCurrent)
	dstOff, _ := dst.Seek(0, io.SeekCurrent)
	if srcOff != 7 {
		t.Errorf("src offset: got %d, want 7", srcOff)
	}
	if dstOff != 13 {
		t.Errorf("dst offset: got %d, want 13", dstOff)
	}

	// content check: dst should contain src from offset 0 (whole-file copy)
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("dst content mismatch after CopyFd")
	}
}
