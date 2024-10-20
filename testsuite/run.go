// trun.go -- do a single test suite

package main

import (
	"fmt"
	"os"
	"path"

	cmp "github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
	"github.com/opencoff/go-logger"
)

// TestEnv captures the runtime environment of the current testsuite
type TestEnv struct {
	Lhs string
	Rhs string

	TestRoot string
	TestName string

	ltree *cmp.Tree
	rtree *cmp.Tree

	log logger.Logger
}

func RunTest(tname string, cfg *config, ts []TestSuite) (err error) {
	if len(ts) < 2 {
		return fmt.Errorf("too few commands in test suite")
	}

	// setup test env
	env, err := makeEnv(tname, cfg)
	if err != nil {
		return err
	}

	defer func(e *error) {
		if *e != nil {
			env.log.Info("test complete: error:\n%s", *e)
		} else {
			env.log.Info("test complete; no errors")
		}
		env.log.Close()
	}(&err)

	// substitute environment vars in each arg
	lookup := map[string]string{
		"LHS":   env.Lhs,
		"RHS":   env.Rhs,
		"ROOT":  env.TestRoot,
		"TNAME": env.TestName,

		// TODO: Other vars in the future
	}

	env.log.Info("testroot %s; starting test %s ..", env.TestRoot, env.TestName)
	for _, t := range ts {
		cmd := t.Cmd

		args := make([]string, 0, len(t.Args))
		for _, s := range t.Args[1:] {
			d := os.Expand(s, func(key string) string {
				v, ok := lookup[key]
				if !ok {
					Die("%s: can't expand env %s", cmd.Name(), key)
				}
				return v
			})
			args = append(args, d)
		}

		cmd.Reset()
		if err = cmd.Run(env, args); err != nil {
			return fmt.Errorf("%s: %s: %w", tname, cmd.Name(), err)
		}
	}

	// cleanup as we go - so we don't accumulate cruft
	if err = os.RemoveAll(env.TestRoot); err != nil {
		Die("%s:  cleanup %s: %w", env.TestName, env.TestRoot, err)
	}

	return nil
}

// make the test environment that's common to each individual test.
func makeEnv(tname string, cfg *config) (*TestEnv, error) {
	tmpdir := path.Join(cfg.tempdir, tname)
	lhs := path.Join(tmpdir, "lhs")
	rhs := path.Join(tmpdir, "rhs")
	logfile := path.Join(tmpdir, "dircmp.log")
	if cfg.logStdout {
		logfile = "STDOUT"
	}

	if err := os.MkdirAll(lhs, 0700); err != nil {
		return nil, fmt.Errorf("%s: LHS: %w", tname, err)
	}

	if err := os.MkdirAll(rhs, 0700); err != nil {
		return nil, fmt.Errorf("%s: RHS: %w", tname, err)
	}

	wo := walk.Options{
		Concurrency: 8,
		Type:        walk.ALL & ^walk.DIR,
	}

	lt, err := cmp.NewTree(lhs, cmp.WithWalkOptions(&wo))
	if err != nil {
		return nil, fmt.Errorf("%s: tree: %w", lhs, err)
	}

	rt, err := cmp.NewTree(rhs, cmp.WithWalkOptions(&wo))
	if err != nil {
		return nil, fmt.Errorf("%s: tree: %w", rhs, err)
	}

	log, err := logger.NewLogger(logfile, logger.LOG_DEBUG, tname, logger.Ldate|logger.Ltime|logger.Lmicroseconds|logger.Lfileloc)
	if err != nil {
		return nil, fmt.Errorf("%s: logfile: %w", tname, err)
	}

	e := &TestEnv{
		Lhs:      lhs,
		Rhs:      rhs,
		TestRoot: tmpdir,
		TestName: tname,
		log:      log,

		ltree: lt,
		rtree: rt,
	}

	return e, nil
}

func (t *TestEnv) String() string {
	s := fmt.Sprintf("TestEnv: name %s: Root: %s\n\tLHS %s, RHS %s\n",
		t.TestName, t.TestRoot, t.Lhs, t.Rhs)
	return s
}
