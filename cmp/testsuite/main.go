// tmain.go - main test runner

package main

import (
	"fmt"
	"os"
	"path"

	"crypto/rand"

	flag "github.com/opencoff/pflag"
)

var Z = path.Base(os.Args[0])

type config struct {
	tempdir string

	// TODO progress bar and other things
	progress  bool
	logStdout bool
}

func main() {

	var progress, help, serial, stdout bool
	var tmpdir string

	fs := flag.NewFlagSet(Z, flag.ExitOnError)

	fs.BoolVarP(&help, "help", "h", false, "Show help and exit [False]")
	fs.BoolVarP(&progress, "progress", "p", false, "Show progress bar [False]")
	fs.StringVarP(&tmpdir, "workdir", "d", "", "Use `D` as the test root directory [OS Tempdir]")
	fs.BoolVarP(&serial, "serial", "s", false, "Run tests serially [False]")
	fs.BoolVarP(&stdout, "log-stdout", "", false, "Put log output to STDOUT [False]")

	fs.SetOutput(os.Stdout)

	err := fs.Parse(os.Args[1:])
	if err != nil {
		Die("%s", err)
	}

	if help {
		usage(fs)
	}

	args := fs.Args()
	if len(args) == 0 {
		Die("Usage: %s test.t [test.t...]", Z)
	}

	tempdir := os.TempDir()
	if len(tmpdir) > 0 {
		tempdir = tmpdir
	}

	tempdir = path.Join(tempdir, "dircmp", randstr(5))
	cfg := &config{
		tempdir:   tempdir,
		progress:  progress,
		logStdout: stdout,
	}

	for _, fn := range args {
		err = runTest(cfg, fn)
		if err != nil {
			break
		}
	}

	if err != nil {
		Die("%s", err)
	}

	// only cleanup tempdir iff no errors
	// Each test will cleanup its own dir if no-error
	err = os.RemoveAll(tempdir)
	if err != nil {
		Die("can't remove tempdir %s: %s", tempdir, err)
	}
}

// Run a single test in file 'fn'
func runTest(cfg *config, fn string) error {
	ts, err := ReadTest(fn)
	if err != nil {
		return err
	}

	tname := path.Base(fn)
	if err = RunTest(tname, cfg, ts); err != nil {
		return err
	}

	return nil
}

func usage(fs *flag.FlagSet) {
	fmt.Printf(usageStr, Z, Z)
	fs.PrintDefaults()
	os.Exit(1)
}

func randstr(n int) string {
	b := make([]byte, n)

	m, err := rand.Read(b[:])
	if err != nil || m != n {
		panic("can't read random bytes from OS")
	}
	return fmt.Sprintf("%x", b)
}

var usageStr = `%s - tarsync test runner.

Tests are descibed in a DSL. Each test must be in a file with a '.t' suffix.
The tests are run in parallel by default (one worker per cpu/core).

Usage: %s [options] test [test...]

Options:
`
