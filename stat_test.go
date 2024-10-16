// stat_test.go - test harness for stat/lstat
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
	"os"
	"path/filepath"
	"testing"
)

func TestStat(t *testing.T) {
	assert := newAsserter(t)
	tmpdir := t.TempDir()

	fp := filepath.Join(tmpdir, "a")
	err := mkfilex(fp)
	assert(err == nil, "mkfile: %s", err)

	st, err := os.Stat(fp)
	assert(err == nil, "os.stat: %s", err)

	fi, err := Stat(fp)
	assert(err == nil, "stat: %s", err)

	err = statEq(st, fi)
	assert(err == nil, "%s", err)
}

func statEq(st os.FileInfo, fi *Info) error {
	if st.Size() != fi.Size() {
		return fmt.Errorf("size: exp %d, saw %d", st.Size(), fi.Size())
	}
	if st.Mode() != fi.Mode() {
		return fmt.Errorf("mode: exp %#b, saw %#b", st.Mode(), fi.Mode())
	}
	return nil
}
