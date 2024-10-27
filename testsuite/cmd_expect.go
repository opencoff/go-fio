// cmd_expect.go -- implements the "expect" command

package main

import (
	"fmt"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
)

type expectCmd struct {
}

func (t *expectCmd) New() Cmd {
	return &expectCmd{}
}

func (t *expectCmd) Name() string {
	return "expect"
}

func (t *expectCmd) Run(env *TestEnv, args []string) error {
	exp := map[string][]string{
		"ld":    {},	// left only dirs
		"lf":    {},	// left only files
		"rd":    {},	// right only dirs
		"rf":    {},	// right only files
		"cd":    {},	// common dirs
		"cf":    {},	// common files
		"diff":  {},	// different files
		"funny": {},	// funny entries
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


	wo := walk.Options{
		Concurrency: env.ncpu,
		Type: walk.ALL,
	}

	// now run the difference engine and collect output
	diff, err := cmp.DirTree(env.Lhs, env.Rhs,
		//cmp.WithIgnoreAttr(cmp.IGN_HARDLINK),
		cmp.WithWalkOptions(wo))
	if err != nil {
		return err
	}

	env.log.Debug(diff.String())

	for k, v := range exp {
		switch k {
		case "ld":
			err = match(k, v,  keys(diff.LeftDirs))
		case "lf":
			err = match(k, v,  keys(diff.LeftFiles))
		case "rd":
			err = match(k, v,  keys(diff.RightDirs))
		case "rf":
			err = match(k, v,  keys(diff.RightFiles))

		case "cd":
			err = match(k, v,  keys(diff.CommonDirs))
		case "cf":
			err = match(k, v,  keys(diff.CommonFiles))
		case "diff":
			err = match(k, v,  keys(diff.Diff))
		case "funny":
			err = match(k, v,  keys(diff.Funny))
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func keys[K comparable, V any](m *xsync.MapOf[K, V]) []K {
	var v []K
	m.Range(func (k K, _ V) bool {
		v = append(v, k)
		return true
	})
	return v
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

var _ Cmd = &expectCmd{}

func init() {
	RegisterCommand(&expectCmd{})
}
