// cmd_mkfile.go -- implements the "tree" command

package main

import (
	"fmt"
	"math/rand/v2"
	"path"
	"time"

	"github.com/opencoff/go-fio"
	flag "github.com/opencoff/pflag"
)

type mkfileCmd struct {
	*flag.FlagSet

	target string
	mkdir  bool
	minsz  SizeValue
	maxsz  SizeValue
}

func (t *mkfileCmd) Name() string {
	return "mkfile"
}

func (t *mkfileCmd) Reset() {
	t.minsz = 1024
	t.maxsz = 8 * 1024
	t.mkdir = false
}

// mkfile [-t target] entries...
func (t *mkfileCmd) Run(env *TestEnv, args []string) error {
	err := t.Parse(args)
	if err != nil {
		return fmt.Errorf("mkfile: %w", err)
	}

	env.log.Debug("mkfile: '%s': sizes: min %d max %d\n", t.target,
		t.minsz.Value(), t.maxsz.Value())

	args = t.Args()
	now := env.Start
	switch t.target {
	case "lhs":
		err = t.mkfile("lhs", args, env, now)
	case "rhs":
		err = t.mkfile("rhs", args, env, now)
	case "both":
		if err = t.mkfile("lhs", args, env, now); err != nil {
			return fmt.Errorf("mkfile: %w", err)
		}

		if err = t.cloneLhs(args, env); err != nil {
			return fmt.Errorf("mkfile: %w", err)
		}
	default:
		return fmt.Errorf("mkfile: unknown target direction '%s'", t.target)
	}

	if err != nil {
		return fmt.Errorf("mkfile: %w", err)
	}
	return nil
}

func (t *mkfileCmd) cloneLhs(args []string, env *TestEnv) error {
	base := env.TestRoot
	for _, nm := range args {
		if path.IsAbs(nm) {
			return fmt.Errorf("common file %s can't be absolute", nm)
		}

		lhs := path.Join(base, "lhs", nm)
		rhs := path.Join(base, "rhs", nm)
		env.log.Debug("mkfile clone %s -> %s", lhs, rhs)
		if err := fio.CloneFile(rhs, lhs); err != nil {
			return fmt.Errorf("%s: %w", rhs, err)
		}
	}
	return nil
}

func (t *mkfileCmd) mkfile(key string, args []string, env *TestEnv, now time.Time) error {
	base := env.TestRoot
	for _, nm := range args {
		var err error
		fn := nm

		if !path.IsAbs(nm) {
			fn = path.Join(base, key, fn)
		}

		if t.mkdir {
			env.log.Debug("mkdir %s", fn)
			err = mkdir(fn, now)
		} else {
			sz := int64(rand.N(t.maxsz-t.minsz) + t.minsz)
			env.log.Debug("mkfile %s %d", fn, sz)
			err = mkfile(fn, sz, now)
		}

		if err != nil {
			return fmt.Errorf("%s: %w", fn, err)
		}
	}
	return nil
}

var _ Cmd = &mkfileCmd{}

func init() {
	tc := &mkfileCmd{
		FlagSet: flag.NewFlagSet("mkfile", flag.ExitOnError),
	}

	fs := tc.FlagSet
	fs.VarP(&tc.minsz, "min-file-size", "m", "Minimum file size to be created [1k]")
	fs.VarP(&tc.maxsz, "max-file-size", "M", "Maximum file size to be created [8k]")
	fs.BoolVarP(&tc.mkdir, "dir", "d", false, "Make directories instead of files")
	fs.StringVarP(&tc.target, "target", "t", "lhs", "Make entries in the given location (lhs, rhs, both)")

	RegisterCommand(tc)
}
