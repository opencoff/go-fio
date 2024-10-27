// cache.go -- a stat/lstat cache
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
	"github.com/opencoff/go-fio"
)

type statCache struct {
	lstat *FioMap
}

func newStatCache() *statCache {
	cc := &statCache{
		lstat: newMap(),
	}
	return cc
}

// purge the cache
func (cc *statCache) Clear() {
	cc.lstat.Clear()
}

func (cc *statCache) StoreLstat(fi *fio.Info) {
	cc.lstat.Store(fi.Name(), fi)
}

func (cc *statCache) Lstat(nm string) (*fio.Info, error) {
	// XXX there is no atomic "get or set" function
	fi, ok := cc.lstat.Load(nm)
	if ok {
		return fi, nil
	}

	// put a new instance
	fi, err := fio.Lstat(nm)
	if err != nil {
		return nil, err
	}

	fi, _ = cc.lstat.LoadOrStore(nm, fi)
	return fi, nil
}
