// engine.go - An engine for comparing metadata from two similar Filesys trees.
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

	"github.com/opencoff/go-fio"
)

type work struct {
	nm string
	fi *fio.Info
}

func (c *cmp) doDiff() error {
	wp := fio.NewWorkPool[work](c.Concurrency, func(i int, w work) error {
		c.lhsDiff(w.nm, w.fi)
		return nil
	})

	c.lhs.Range(func(nm string, fi *fio.Info) bool {
		w := work{nm, fi}
		wp.Submit(w)
		return true
	})
	wp.Close()

	if err := wp.Wait(); err != nil {
		return err
	}

	// Process the rhs only after we've done the left side;
	// we need the done and funny maps to be complete before
	// we do this.
	wp = fio.NewWorkPool[work](c.Concurrency, func(i int, w work) error {
		c.rhsDiff(w.nm, w.fi)
		return nil
	})
	c.rhs.Range(func(nm string, fi *fio.Info) bool {
		w := work{nm, fi}
		wp.Submit(w)
		return true
	})
	wp.Close()

	return wp.Wait()
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
