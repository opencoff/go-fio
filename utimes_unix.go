// utimes_unix.go -- set file times for unixish platforms
//
// (c) 2021 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build unix

package fio

import (
	"io/fs"
	"os"
)

func clonetimes(dest string, fi *Info) error {

	// The situation with utimes and symlinks is broken across
	// platforms:
	//  - darwin and bsd's don't have nano-second utimes() or lutimes()
	//  - linux has 4 differnt variants of utimes/lutimes/utimensat etc.
	//  - then there is the confusing mess of struct timespec vs. struct timeval
	//    (one has ns resolution while the other has us).
	//
	//  So for now we ignore symlinks and atime/mtime
	if fi.Mode().Type() != fs.ModeSymlink {
		if err := os.Chtimes(dest, fi.Atim, fi.Mtim); err != nil {
			return err
		}
	}
	return nil
}
