// xattr_filter_darwin.go -- drop macOS ACL xattr from list / set paths.
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

//go:build darwin

package fio

// darwinACLKey is the reserved extended-attribute name under which
// macOS stores POSIX ACLs. The blob is opaque and the supported
// API for reading / writing it is acl_get_file(3) and
// acl_set_file(3); raw setxattr against this key does NOT round-
// trip correctly (the kernel expects a versioned/signed payload).
//
// To keep clone safe on darwin we filter this key out of both
// read and write paths. ACL round-tripping on darwin is a known
// limitation - callers that need it must use acl_*(3) directly.
const darwinACLKey = "com.apple.system.Security"

// isFilteredXattr reports whether the key is in a platform-
// specific "do not round-trip via setxattr" list.
func isFilteredXattr(key string) bool {
	return key == darwinACLKey
}

// filterKeys returns keys with filtered entries removed. The input
// slice is reused (in-place compaction); callers that still need
// the full list must copy before calling.
func filterKeys(keys []string) []string {
	out := keys[:0]
	for _, k := range keys {
		if isFilteredXattr(k) {
			continue
		}
		out = append(out, k)
	}
	return out
}
