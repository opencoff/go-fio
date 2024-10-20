// cmd_mkfile.go -- implements the "tree" command

package main

import (
	"fmt"
	"math/rand/v2"
	"path"
	"time"

	flag "github.com/opencoff/pflag"
)

type mkfileCmd struct {
	*flag.FlagSet

	mkdir bool
	minsz SizeValue
	maxsz SizeValue
}

func (t *mkfileCmd) Name() string {
	return "mkfile"
}

func (t *mkfileCmd) Reset() {
	t.minsz = 1024
	t.maxsz = 128 * 1024
	t.mkdir = false
}

// mkfile [options] lhs=" ..."  rhs="..."
func (t *mkfileCmd) Run(env *TestEnv, args []string) error {
	err := t.Parse(args)
	if err != nil {
		return fmt.Errorf("mkfile: %w", err)
	}

	args = t.Args()

	env.log.Debug("mkfile: sizes: min %d max %d\n",
		t.minsz.Value(), t.maxsz.Value())

	n := 0
	for i := range args {
		arg := args[i]

		key, vals, err := Split(arg)
		if err != nil {
			return err
		}

		if key != "lhs" && key != "rhs" {
			return fmt.Errorf("mkfile: unknown keyword %s", key)
		}

		if len(vals) == 0 {
			return fmt.Errorf("mkfile: %s is empty?", key)
		}

		n += len(vals)
		if err = t.mkfile(key, vals, env); err != nil {
			return fmt.Errorf("mkfile: %w", err)
		}
	}

	if n == 0 {
		return fmt.Errorf("mkfile: no entries to create")
	}
	return nil
}

func (t *mkfileCmd) mkfile(key string, vals []string, env *TestEnv) error {
	base := env.TestRoot
	now := time.Now().UTC()
	for _, nm := range vals {
		var err error

		if !path.IsAbs(nm) {
			nm = path.Join(base, key, nm)
		}

		if t.mkdir {
			env.log.Debug("mkdir %s", nm)
			err = mkdir(nm, now)
		} else {
			sz := int64(rand.N(t.maxsz-t.minsz) + t.minsz)
			env.log.Debug("mkfile %s %d", nm, sz)
			err = mkfile(nm, sz, now)
		}

		if err != nil {
			return fmt.Errorf("mkfile: %s: %w", nm, err)
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
	fs.VarP(&tc.maxsz, "max-file-size", "M", "Maximum file size to be created [1M]")
	fs.BoolVarP(&tc.mkdir, "dir", "d", false, "Make directories instead of files")

	RegisterCommand(tc)
}
