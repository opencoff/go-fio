// cmd_expect.go -- implements the "expect" command
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim is made to its
// suitability for any purpose.

package cmp_test

import (
	"context"
	"fmt"

	"github.com/opencoff/go-fio/cmp"
	"github.com/opencoff/go-fio/walk"
	tr "github.com/opencoff/go-testrunner"
	"github.com/puzpuzpuz/xsync/v4"
)

type expectCmd struct {
}

func (t *expectCmd) Name() string {
	return "expect"
}

func (t *expectCmd) Run(env *tr.TestEnv, args []string) error {
	exp := map[string][]string{
		"ld":    {}, // left only dirs
		"lf":    {}, // left only files
		"rd":    {}, // right only dirs
		"rf":    {}, // right only files
		"cd":    {}, // common dirs
		"cf":    {}, // common files
		"diff":  {}, // different files
		"funny": {}, // funny entries
	}

	for i := range args {
		arg := args[i]

		key, vals, err := tr.Split(arg)
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
		Concurrency: env.Ncpu,
		Type:        walk.ALL,
	}

	// now run the difference engine and collect output
	diff, err := cmp.FsTree(context.Background(), env.Lhs, env.Rhs, cmp.WithWalkOptions(wo))
	if err != nil {
		return err
	}

	env.Log.Debug(diff.String())

	for k, v := range exp {
		switch k {
		case "ld":
			err = match(k, v, keys(diff.LeftDirs))
		case "lf":
			err = match(k, v, keys(diff.LeftFiles))
		case "rd":
			err = match(k, v, keys(diff.RightDirs))
		case "rf":
			err = match(k, v, keys(diff.RightFiles))

		case "cd":
			err = match(k, v, keys(diff.CommonDirs))
		case "cf":
			err = match(k, v, keys(diff.CommonFiles))
		case "diff":
			err = match(k, v, keys(diff.Diff))
		case "funny":
			err = match(k, v, keys(diff.Funny))
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func keys[K comparable, V any](m *xsync.Map[K, V]) []K {
	var v []K
	for k := range m.All() {
		v = append(v, k)
	}
	return v
}

func match(key string, exp, have []string) error {
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

var _ tr.Cmd = &expectCmd{}

func init() {
	tr.RegisterCommand(&expectCmd{})
}
