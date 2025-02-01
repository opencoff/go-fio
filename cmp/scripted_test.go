// scripted_test.go -- script driven clone & cmp dir tests
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

package cmp_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tr "github.com/opencoff/go-testrunner"
)

func TestTreeScript(t *testing.T) {
	assert := newAsserter(t)
	tmpdir := getTmpdir(t)

	args := flag.Args()
	//fmt.Printf("args: %v\n", args)

	if len(args) == 0 {
		av, err := readdir("./tests")
		assert(err == nil, "readdir tests: %s", err)
		args = av[:0]
		for _, nm := range av {
			if strings.HasSuffix(nm, ".t") {
				args = append(args, nm)
			}
		}
	}

	if len(args) == 0 {
		return
	}

	cfg := &tr.Config{
		Tempdir: tmpdir,
		Ncpu:    runtime.NumCPU() / 2,
	}

	r := tr.New(cfg)
	for _, nm := range args {
		t.Logf("%s ...\n", nm)
		err := r.RunOne(nm)
		assert(err == nil, "%s", err)
	}
}

func readdir(nm string) ([]string, error) {
	fd, err := os.Open(nm)
	if err != nil {
		return nil, fmt.Errorf("opendir %s: %w", nm, err)
	}
	defer fd.Close()

	names, err := fd.Readdirnames(-1)
	if err != nil {
		return nil, fmt.Errorf("readdir %s: %w", nm, err)
	}

	z := names[:0]
	for _, a := range names {
		z = append(z, filepath.Join(nm, a))
	}
	return z, nil
}
