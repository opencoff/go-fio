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
	"fmt"
	"os"
)

func utimes(dest string, _ string, fi *Info) error {
	if err := os.Chtimes(dest, fi.Atim, fi.Mtim); err != nil {
		return fmt.Errorf("utimes: %w", err)
	}
	return nil
	/*
		tv := []unix.Timeval{
			unix.NsecToTimeval(fi.Atim.Nano()),
			unix.NsecToTimeval(fi.Mtim.Nano()),
		}

		if err := unix.Lutimes(dest, tv); err != nil {
			return fmt.Errorf("utimes: set: %w", err)
		}
		return nil
	*/
}
