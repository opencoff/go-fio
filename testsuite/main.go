// tmain.go - main test runner

package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"

	"crypto/rand"

	flag "github.com/opencoff/pflag"
)

var Z = path.Base(os.Args[0])

type config struct {
	tempdir string

	// TODO progress bar and other things
	progress  bool
	logStdout bool
	ncpu      int
}

func main() {

	var progress, help, serial, stdout bool
	var tmpdir string
	var ncpu int

	fs := flag.NewFlagSet(Z, flag.ExitOnError)

	fs.BoolVarP(&help, "help", "h", false, "Show help and exit [False]")
	fs.BoolVarP(&progress, "progress", "p", false, "Show progress bar [False]")
	fs.StringVarP(&tmpdir, "workdir", "d", "", "Use `D` as the test root directory [OS Tempdir]")
	fs.BoolVarP(&serial, "serial", "s", false, "Run tests serially [False]")
	fs.BoolVarP(&stdout, "log-stdout", "", false, "Put log output to STDOUT [False]")
	fs.IntVarP(&ncpu, "concurrency", "c", runtime.NumCPU(), "Use upto `N` CPUs for all tests")

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

	tempdir = path.Join(tempdir, "dircmp", randstr(4))
	cfg := &config{
		tempdir:   tempdir,
		progress:  progress,
		logStdout: stdout,
		ncpu:      ncpu,
	}

	if serial {
		err = serialize(cfg, args)
	} else {
		err = parallelize(cfg, args)
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

func serialize(cfg *config, args []string) error {
	for _, fn := range args {
		if err := runTest(cfg, fn); err != nil {
			return err
		}
	}
	return nil
}

func parallelize(cfg *config, args []string) error {
	ch := make(chan string, cfg.ncpu)
	ech := make(chan error, 1)

	var ewg, wg sync.WaitGroup
	var errs []error

	// harvest errors
	ewg.Add(1)
	go func() {
		for e := range ech {
			errs = append(errs, e)
		}
		ewg.Done()
	}()

	// queue up work for the workers; this goroutine _will_
	// end; no need to use a waitgroup
	go func() {
		for _, fn := range args {
			ch <- fn
		}
		// and tell workers that we're done
		close(ch)
	}()

	// start workers
	wg.Add(cfg.ncpu)
	for i := 0; i < cfg.ncpu; i++ {
		go func(wg *sync.WaitGroup) {
			for fn := range ch {
				if err := runTest(cfg, fn); err != nil {
					ech <- err
				}
			}
			wg.Done()
		}(&wg)
	}

	// wait for them to complete
	wg.Wait()

	// then complete harvesting all errors
	close(ech)
	ewg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
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
