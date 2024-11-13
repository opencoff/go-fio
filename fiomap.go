// fiomap.go -- a map of names to Info
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

package fio

import (
	"github.com/puzpuzpuz/xsync/v3"
)

// Pair represents the Stat/Lstat info of a pair of
// related file system entries in the source and destination
type Pair struct {
	Src, Dst *Info
}

// FioMap is a concurrency safe map of relative path name and the
// corresponding Stat/Lstat info.
type FioMap = xsync.MapOf[string, *Info]

// FioPairMap is a concurrency safe map of relative path name and the
// corresponding Stat/Lstat info of both the source and destination.
type FioPairMap = xsync.MapOf[string, Pair]

func NewFioMap() *FioMap {
	return xsync.NewMapOf[string, *Info]()
}

func NewFioPairMap() *FioPairMap {
	return xsync.NewMapOf[string, Pair]()
}
