// cmd_expect.go -- implements the "expect" command

package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opencoff/go-fio/clone"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
	"github.com/puzpuzpuz/xsync/v3"
)

type cloneCmd struct {
}

func (t *cloneCmd) New() Cmd {
	return &cloneCmd{}
}

func (t *cloneCmd) Name() string {
	return "clone"
}

func (t *cloneCmd) Run(env *TestEnv, args []string) error {
	var funny []string

	// gather any args
	for _, arg := range args {
		key, vals, err := Split(arg)
		if err != nil {
			return err
		}
		if key != "funny" {
			return fmt.Errorf("clone: unknown keyword %s", key)
		}
		if len(vals) > 0 {
			funny = append(funny, vals...)
		}
	}

	wo := walk.Options{
		Concurrency: env.ncpu,
		Type:        walk.ALL,
	}

	err := clone.Tree(env.Rhs, env.Lhs, clone.WithWalkOptions(wo))
	if err != nil {
		var ferr *clone.FunnyError
		if errors.As(err, &ferr) && len(funny) > 0 {
			return matchFunny(ferr, funny)
		}

		return err
	}

	// now run the difference engine and collect output
	diff, err := cmp.FsTree(env.Lhs, env.Rhs, cmp.WithWalkOptions(wo))
	if err != nil {
		return err
	}

	env.log.Debug(diff.String())

	err = treeEq(diff)
	if err != nil {
		return err
	}
	return nil
}

// match funny entries in the error vs. what is expected
func matchFunny(fe *clone.FunnyError, funny []string) error {
	if len(fe.Funny) != len(funny) {
		return fmt.Errorf("funny: exp %d, saw %d entries", len(funny), len(fe.Funny))
	}

	// build a lookup table
	done := make(map[string]bool)
	for i := range fe.Funny {
		ent := &fe.Funny[i]
		done[ent.Name] = false
	}

	for _, nm := range funny {
		_, ok := done[nm]
		if !ok {
			return fmt.Errorf("funny: missing %s", nm)
		}
		done[nm] = true
	}

	for nm, ok := range done {
		if !ok {
			return fmt.Errorf("funny: saw extra %s", nm)
		}
	}
	return nil
}

func treeEq(d *cmp.Difference) error {
	if d.Funny.Size() > 0 {
		return xerror("funny", d.Funny)
	}

	if d.LeftDirs.Size() > 0 {
		return xerror("left-dirs", d.LeftDirs)
	}
	if d.LeftFiles.Size() > 0 {
		return xerror("left-files", d.LeftFiles)
	}

	if d.RightDirs.Size() > 0 {
		return xerror("right-dirs", d.RightDirs)
	}
	if d.RightFiles.Size() > 0 {
		return xerror("right-files", d.RightFiles)
	}

	if d.Diff.Size() > 0 {
		return xerror("diff", d.Diff)
	}
	return nil
}

func xerror[K string, V any](pref string, m *xsync.MapOf[string, V]) error {
	var b strings.Builder

	fmt.Fprintf(&b, "%s:\n", pref)
	m.Range(func(nm string, _ V) bool {
		fmt.Fprintf(&b, "\t%s\n", nm)
		return true
	})

	return fmt.Errorf("error - %s", b.String())
}

var _ Cmd = &cloneCmd{}

func init() {
	RegisterCommand(&cloneCmd{})
}
