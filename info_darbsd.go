// info_darbsd.go - syscall.Stat_t to Info for darwin and freebsd
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build darwin || freebsd

package fio

import (
	"io/fs"
	"syscall"
)

func makeInfo(fi *Info, nm string, st *syscall.Stat_t, x Xattr) {
	*fi = Info{
		Ino:    st.Ino,
		Siz:    st.Size,
		Blocks: int64(st.Blocks),
		// st.Dev/st.Rdev are int32 on darwin and uint64 on
		// freebsd. Real device IDs are non-negative; the
		// uint32 step makes the zero-extension explicit on
		// darwin and is a no-op on freebsd.
		Dev:  uint64(uint32(st.Dev)),  // #nosec G115 -- zero-extend darwin int32 dev_t
		Rdev: uint64(uint32(st.Rdev)), // #nosec G115 -- zero-extend darwin int32 dev_t

		Mod:   fs.FileMode(st.Mode & 0777),
		Uid:   st.Uid,
		Gid:   st.Gid,
		Nlink: uint32(st.Nlink),

		Atim: ts2time(st.Atimespec),
		Mtim: ts2time(st.Mtimespec),
		Ctim: ts2time(st.Ctimespec),

		path:  nm,
		Xattr: x,
	}

	switch st.Mode & syscall.S_IFMT {
	case syscall.S_IFBLK:
		fi.Mod |= fs.ModeDevice
	case syscall.S_IFCHR:
		fi.Mod |= fs.ModeDevice | fs.ModeCharDevice
	case syscall.S_IFDIR:
		fi.Mod |= fs.ModeDir
	case syscall.S_IFIFO:
		fi.Mod |= fs.ModeNamedPipe
	case syscall.S_IFLNK:
		fi.Mod |= fs.ModeSymlink
	case syscall.S_IFREG:
		// nothing to do
	case syscall.S_IFSOCK:
		fi.Mod |= fs.ModeSocket
	}
	if st.Mode&syscall.S_ISGID != 0 {
		fi.Mod |= fs.ModeSetgid
	}
	if st.Mode&syscall.S_ISUID != 0 {
		fi.Mod |= fs.ModeSetuid
	}
	if st.Mode&syscall.S_ISVTX != 0 {
		fi.Mod |= fs.ModeSticky
	}
}
