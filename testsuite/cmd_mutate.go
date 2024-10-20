// cmd_mutate.go -- implements the "mutate" command

package main

import (
	"fmt"
	"path"
)

type mutateCmd struct {
}

const (
	// % of file to be mutated
	minMutation int64 = 10
	maxMutation int64 = 30
)

func (t *mutateCmd) Reset() {
}

func (t *mutateCmd) Run(env *TestEnv, args []string) error {
	for i := range args {
		arg := args[i]

		key, vals, err := Split(arg)
		if err != nil {
			return err
		}

		if key != "lhs" && key != "rhs" {
			return fmt.Errorf("mutate: unknown keyword %s", key)
		}

		if len(vals) == 0 {
			return fmt.Errorf("mutate: %s is empty?", key)
		}

		if err = t.mutate(key, vals, env); err != nil {
			return fmt.Errorf("mutate: %w", err)
		}
	}
	return nil
}

func (t *mutateCmd) mutate(key string, vals []string, env *TestEnv) error {
	base := env.TestRoot
	for _, nm := range vals {
		if !path.IsAbs(nm) {
			nm = path.Join(base, key, nm)
		}

		if exists, err := FileExists(nm); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("%s: doesn't exist", nm)
		}

		env.log.Debug("mutate %s", nm)

		if err := mutate(nm, minMutation, maxMutation); err != nil {
			return fmt.Errorf("%s: %w", nm, err)
		}
	}
	return nil
}

func (t *mutateCmd) Name() string {
	return "mutate"
}

var _ Cmd = &mutateCmd{}

func init() {
	// mutate takes no args
	RegisterCommand(&mutateCmd{})
}
