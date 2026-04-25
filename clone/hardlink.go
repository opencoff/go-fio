// hardlink.go -- tracking & cloning hardlinks
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

// go:build unix

package clone

import (
	"fmt"

	"github.com/opencoff/go-fio"
	"github.com/puzpuzpuz/xsync/v4"
)

// We track hardlinked files using the src file's properties.
// Only the source knows how many hardlinks the cloner must
// create. The first time we enocunter a destination that must have
// more than 1 hard link, we track it in 'm'. Subsequent hardlinks
// to the same inode result in tracking the _new_ hardlink name
// against the first one; this is tracked in 'links'.

// linkRec captures the orig dst path together with the
// src-side *fio.Info; observers want both.
type linkRec struct {
	origDst string
	src     *fio.Info
}

type hardlinker struct {
	// tracks src:inode -> orig_dst
	m *xsync.Map[string, string]

	// stores the map of new_dst -> {orig_dst, src_info}
	links *xsync.Map[string, linkRec]
}

func newHardlinker() *hardlinker {
	h := &hardlinker{
		m:     xsync.NewMap[string, string](),
		links: xsync.NewMap[string, linkRec](),
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
	// Atomically decide who is the "original" for this inode.
	// A racing Load+Store pair would let two goroutines both
	// claim !ok and both end up copying; LoadOrStore collapses
	// that decision into a single atomic step.
	orig, loaded := h.m.LoadOrStore(k, dst)
	if loaded {
		h.links.Store(dst, linkRec{origDst: orig, src: src})
		return true
	}

	// We are the first caller for this inode; the copy will be
	// done by the caller, and subsequent callers will record
	// hardlinks against 'dst'.
	return false
}

func (h *hardlinker) hardlinks(fp func(dst, src string, fi *fio.Info)) {
	// k == dst, v.origDst == orig src, v.src == src Info
	for k, v := range h.links.All() {
		fp(k, v.origDst, v.src)
	}
}
