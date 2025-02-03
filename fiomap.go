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

// Map is a concurrency safe map of path name and the
// corresponding Stat/Lstat info.
type Map = xsync.MapOf[string, *Info]

// PairMap is a concurrency safe map of path name and the
// corresponding Stat/Lstat info of both the source and destination.
type PairMap = xsync.MapOf[string, Pair]

// NewMap makes a new concurrent map of name to stat/lstat info
func NewMap() *Map {
	return xsync.NewMapOf[string, *Info]()
}

// NewPairMap makes a new concurrent map of name to a Pair
func NewPairMap() *PairMap {
	return xsync.NewMapOf[string, Pair]()
}
