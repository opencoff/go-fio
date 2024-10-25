// cmd_sync.go -- implements the "sync" command

package main

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/opencoff/go-fio"
	"github.com/opencoff/go-fio/walk"
)

type syncCmd struct {
}

func (t *syncCmd) New() Cmd {
	return &syncCmd{}
}

func (t *syncCmd) Run(env *TestEnv, args []string) error {
	wo := walk.Options{
		Concurrency: 8,
		Type:        walk.ALL & ^walk.DIR,
	}

	dirs := []string{
		env.Lhs,
		env.Rhs,
	}

	// first adjtime for all non-dir entries
	now := env.Start
	err := walk.WalkFunc(dirs, wo, func(fi *fio.Info) error {
		if fi.Mode().Type() != fs.ModeSymlink {
			err := os.Chtimes(fi.Name(), now, now)
			if err != nil {
				return fmt.Errorf("adjtime: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// now fixup the entries for dirs
	wo.Type = walk.DIR
	err = walk.WalkFunc(dirs, wo, func(fi *fio.Info) error {
		err := os.Chtimes(fi.Name(), now, now)
		if err != nil {
			return fmt.Errorf("adjtime: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	return nil
}

func (t *syncCmd) Name() string {
	return "sync"
}

var _ Cmd = &syncCmd{}

func init() {
	RegisterCommand(&syncCmd{})
}
