// hardlink.go -- tracking & cloning hardlinks
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

// go:build unix

package clone

import (
	"fmt"

	"github.com/opencoff/go-fio"
	"github.com/puzpuzpuz/xsync/v3"
)

// We track hardlinked files using the src file's properties.
// Only the source knows how many hardlinks the cloner must
// create. The first time we enocunter a destination that must have
// more than 1 hard link, we track it in 'm'. Subsequent hardlinks
// to the same inode result in tracking the _new_ hardlink name
// against the first one; this is tracked in 'links'.

type hardlinker struct {
	// tracks src:inode -> orig_dst
	m *xsync.MapOf[string, string]

	// stores the map of new_dst -> orig_dst
	links *xsync.MapOf[string, string]
}

func newHardlinker() *hardlinker {
	h := &hardlinker{
		m:     xsync.NewMapOf[string, string](),
		links: xsync.NewMapOf[string, string](),
	}
	return h
}

func key(fi *fio.Info) string {
	return fmt.Sprintf("%d:%d:%d", fi.Dev, fi.Rdev, fi.Ino)
}

func (h *hardlinker) track(src *fio.Info, dst string) bool {
	if src.Nlink == 1 || !src.IsRegular() {
		return false
	}

	k := key(src)
	orig, ok := h.m.Load(k)
	if ok {
		h.links.Store(dst, orig)
		return true
	}

	// remember to do the links after all the copies are done.
	h.m.Store(k, dst)
	return false
}

func (h *hardlinker) hardlinks(fp func(dst, src string)) {
	h.links.Range(func(k, v string) bool {
		// k == dst
		// v == orig src
		fp(k, v)
		return true
	})
}
