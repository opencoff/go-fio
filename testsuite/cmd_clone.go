// cmd_clone.go -- implements the "clone" command to clone dir trees

package main

import (
	"github.com/opencoff/go-fio/clone"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
)

type cloneCmd struct {
}

func (t *cloneCmd) Reset() {
}

// clone - takes no options and invokes clone.Tree()
func (t *cloneCmd) Run(env *TestEnv, args []string) error {
	wo := walk.Options{
		Concurrency: 8,
		Type:        walk.ALL & ^walk.DIR,
	}

	err := clone.Tree(env.Rhs, env.Lhs, clone.WithWalkOptions(wo),
		clone.WithIgnoreAttr(cmp.IGN_HARDLINK))

	return err
}

func (t *cloneCmd) Name() string {
	return "clone"
}

var _ Cmd = &cloneCmd{}

func init() {
	RegisterCommand(&cloneCmd{})
}
