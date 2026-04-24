// xattr_caps_other.go -- capability preflight stubs for non-Linux.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build !linux

package fio

// checkXattrCapabilities is a no-op on non-Linux platforms. POSIX
// capabilities are a Linux concept; other unixes gate xattr writes
// on ownership and kernel-specific rules, so we defer the error
// to the kernel rather than guess.
func checkXattrCapabilities(_ Xattr) error {
	return nil
}
