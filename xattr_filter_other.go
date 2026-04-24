// xattr_filter_other.go -- no-op xattr filter for non-darwin platforms.
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

//go:build !darwin

package fio

// isFilteredXattr is always false on non-darwin platforms. Linux
// and the BSDs do not reserve a parallel "opaque ACL xattr"
// needing this kind of filtering.
func isFilteredXattr(_ string) bool { return false }

// filterKeys is a no-op on non-darwin platforms.
func filterKeys(keys []string) []string { return keys }
