// cmd_expect.go -- implements the "expect" command

package main

import (
	"fmt"
	"path"

	"github.com/opencoff/go-fio/cmp"
)

type expectCmd struct {
}

func (t *expectCmd) Reset() {
}

func (t *expectCmd) Run(env *TestEnv, args []string) error {
	exp := map[string][]string{
		"lo":    {},
		"ro":    {},
		"diff":  {},
		"same":  {},
		"funny": {},
	}

	for i := range args {
		arg := args[i]

		key, vals, err := Split(arg)
		if err != nil {
			return err
		}

		_, ok := exp[key]
		if !ok {
			return fmt.Errorf("expect: unknown keyword %s", key)
		}

		if len(vals) > 0 {
			exp[key] = append(exp[key], vals...)
		}
	}

	// now run the difference engine and collect output
	diff, err := cmp.DirCmp(env.ltree, env.rtree)
	if err != nil {
		return err
	}

	env.log.Debug("Differences:\n%s\n", diff)

	for k, v := range exp {
		switch k {
		case "lo":
			err = match(k, v, diff.LeftOnly)
		case "ro":
			err = match(k, v, diff.RightOnly)
		case "diff":
			err = match(k, v, diff.Diff)
		case "same":
			err = match(k, v, diff.Same)
		case "funny":
			err = match(k, v, diff.Funny)
		}
		if err != nil {
			return fmt.Errorf("expect: %w", err)
		}
	}

	return nil
}

func match(key string, exp, have []string) error {
	if len(exp) != len(have) {
		return fmt.Errorf("%s: exp %d entries, have %d", key, len(exp), len(have))
	}

	mkmap := func(v []string) map[string]bool {
		m := make(map[string]bool)
		for _, nm := range v {
			m[nm] = true
		}
		return m
	}

	e := mkmap(exp)
	h := mkmap(have)

	// every element in have must be in exp
	for _, nm := range have {
		if _, ok := e[nm]; !ok {
			return fmt.Errorf("%s: missing %s", key, nm)
		}
	}

	// every element in exp must be in have
	for _, nm := range exp {
		if _, ok := h[nm]; !ok {
			return fmt.Errorf("%s exp to see %s", key, nm)
		}
	}
	return nil
}

func (t *expectCmd) mutate(key string, vals []string, env *TestEnv) error {
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

func (t *expectCmd) Name() string {
	return "expect"
}

var _ Cmd = &expectCmd{}

func init() {
	RegisterCommand(&expectCmd{})
}
