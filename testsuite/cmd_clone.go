// cmd_expect.go -- implements the "expect" command

package main

import (
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
	wo := walk.Options{
		Concurrency: env.ncpu,
		Type:        walk.ALL,
	}

	err := clone.Tree(env.Rhs, env.Lhs,
		clone.WithIgnoreAttr(cmp.IGN_HARDLINK),
		clone.WithWalkOptions(wo),
	)

	if err != nil {
		return err
	}

	// now run the difference engine and collect output
	diff, err := cmp.DirTree(env.Lhs, env.Rhs,
		cmp.WithIgnoreAttr(cmp.IGN_HARDLINK),
		cmp.WithWalkOptions(wo))
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
