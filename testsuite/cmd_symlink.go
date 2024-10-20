// cmd_symlink.go -- implements the "symlink" command

package main

import (
	"fmt"
	"os"
	"path"
	"strings"
)

type symlinkCmd struct {
}

func (t *symlinkCmd) Reset() {
}

// symlink lhs="newname@oldname newname@oldname" rhs="newname@oldname"
func (t *symlinkCmd) Run(env *TestEnv, args []string) error {
	for i := range args {
		arg := args[i]

		key, vals, err := Split(arg)
		if err != nil {
			return err
		}

		if key != "lhs" && key != "rhs" {
			return fmt.Errorf("symlink: unknown keyword %s", key)
		}

		if len(vals) == 0 {
			return fmt.Errorf("symlink: %s is empty?", key)
		}

		if err = t.symlink(key, vals, env); err != nil {
			return fmt.Errorf("symlink: %w", err)
		}
	}
	return nil
}

func (t *symlinkCmd) symlink(key string, vals []string, env *TestEnv) error {
	base := path.Join(env.TestRoot, key)

	for _, nm := range vals {
		i := strings.Index(nm, "@")
		if i < 0 {
			return fmt.Errorf("symlink: %s: incorrect format; exp NEWNAME@OLDNAME", nm)
		}

		newnm := nm[:i]
		oldnm := nm[i:]

		if !path.IsAbs(oldnm) {
			oldnm = path.Join(base, oldnm)
		}

		if exists, err := FileExists(oldnm); err != nil {
			return err
		} else if !exists {
			return fmt.Errorf("%s: doesn't exist", oldnm)
		}

		env.log.Debug("symlink %s --> %s", oldnm, newnm)
		if err := os.Symlink(oldnm, newnm); err != nil {
			return err
		}
	}
	return nil
}

func (t *symlinkCmd) Name() string {
	return "symlink"
}

var _ Cmd = &symlinkCmd{}

func init() {
	// symlink takes no args
	RegisterCommand(&symlinkCmd{})
}
