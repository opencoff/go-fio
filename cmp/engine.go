// engine.go - An engine for comparing metadata from two similar Filesys trees.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2024 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

package cmp

import (
	"io/fs"
	"runtime"

	"github.com/opencoff/go-fio"
	"golang.org/x/sync/errgroup"
)

func (c *cmp) doDiff() error {
	conc := c.Concurrency
	if conc <= 0 {
		conc = runtime.NumCPU()
	}

	var eg errgroup.Group
	eg.SetLimit(conc)

	c.lhs.Range(func(nm string, fi *fio.Info) bool {
		eg.Go(func() error {
			c.lhsDiff(nm, fi)
			return nil
		})
		return true
	})
	if err := eg.Wait(); err != nil {
		return err
	}

	// Process the rhs only after we've done the left side; rhsDiff
	// reads c.done and c.funny, which lhsDiff populates.
	var eg2 errgroup.Group
	eg2.SetLimit(conc)

	c.rhs.Range(func(nm string, fi *fio.Info) bool {
		eg2.Go(func() error {
			c.rhsDiff(nm, fi)
			return nil
		})
		return true
	})
	return eg2.Wait()
}

func (c *cmp) lhsDiff(nm string, lhs *fio.Info) {
	c.o.VisitSrc(lhs)

	rhs, ok := c.rhs.Load(nm)
	if !ok {
		if lhs.IsDir() {
			c.lhsDir.Store(nm, lhs)
		} else {
			c.lhsFile.Store(nm, lhs)
		}
		return
	}

	// we have two similar named entries on both sides
	pair := fio.Pair{Src: lhs, Dst: rhs}

	// if the file types don't match - skip
	if (lhs.Mod & ^fs.ModePerm) != (rhs.Mod & ^fs.ModePerm) {
		c.funny.Store(nm, pair)
		return
	}

	c.done.Store(nm, true)

	if lhs.IsRegular() {
		if lhs.Size() != rhs.Size() {
			c.diff.Store(nm, pair)
			return
		}
	}

	if eq, _ := c.fileEq(lhs, rhs); !eq {
		c.diff.Store(nm, pair)
		return
	}

	if lhs.IsDir() {
		c.commonDir.Store(nm, pair)
	} else {
		c.commonFile.Store(nm, pair)
	}
}

func (c *cmp) rhsDiff(nm string, rhs *fio.Info) {
	c.o.VisitDst(rhs)

	if _, ok := c.done.Load(nm); ok {
		return
	}

	if _, ok := c.funny.Load(nm); ok {
		return
	}

	// this entry only exists in dst;
	if rhs.IsDir() {
		c.rhsDir.Store(nm, rhs)
	} else {
		c.rhsFile.Store(nm, rhs)
	}
}
