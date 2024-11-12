// hardlink_windows.go -- tracking & cloning hardlinks for windows
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

// go:build windows

package clone

import (
	"github.com/opencoff/go-fio"
)

// We don't support hardlinks on windows
type hardlinker struct{}

func newHardlinker() *hardlinker {
	return &hardlinker{}
}

func (h *hardlinker) track(src *fio.Info, dst string) bool {
	return false
}

func (h *hardlinker) hardlinks(fp func(dst, src string)) {}
