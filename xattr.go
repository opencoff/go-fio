// xattr.go - extended attribute support
//
// (c) 2023- Sudhi Herle <sudhi@herle.net>
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
	"errors"
	"fmt"
	"strings"
	"syscall"

	"github.com/pkg/xattr"
)

// wrapUnsupported tags an ENOTSUP/EOPNOTSUPP error coming out of the
// xattr subsystem as ErrXattrUnsupported so callers that clone onto
// filesystems without xattr support (FAT, tmpfs w/o user_xattr, NFS
// without attr option, etc.) can detect and downgrade via
// errors.Is(err, fio.ErrXattrUnsupported). Other errors pass through.
func wrapUnsupported(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP) {
		// double-%w preserves both the sentinel and the underlying
		// syscall.Errno in the error chain, so callers can errors.Is
		// either one.
		return fmt.Errorf("%w: %w", ErrXattrUnsupported, err)
	}
	return err
}

// Xattr is a collection of all the extended attributes of a given file
type Xattr map[string]string

// String returns the string representation of all the extended attributes
func (x Xattr) String() string {
	var s strings.Builder
	for k, v := range x {
		s.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return s.String()
}

// Equal returns true if all xattr of 'x' is the same as all the
// xattr of 'y' and returns false otherwise.
func (x Xattr) Equal(y Xattr) bool {
	done := make(map[string]bool, len(x))
	for x, a := range x {
		done[x] = true
		if b, ok := y[x]; !ok {
			return false
		} else if a != b {
			return false
		}
	}

	for y := range x {
		if _, ok := done[y]; !ok {
			return false
		}
	}
	return true
}

// GetXattr returns all the extended attributes of a file.
// This function will traverse symlinks.
func GetXattr(nm string) (Xattr, error) {
	return fetch(nm, xattr.List, xattr.Get)
}

// LGetXattr returns all the extended attributes of a file.
// If 'nm' points to a symlink, LGetXattr will return the
// extended attributes of the symlink and *not* the target.
func LgetXattr(nm string) (Xattr, error) {
	return fetch(nm, xattr.LList, xattr.LGet)
}

// SetXattr sets/updates the xattr list for a given file.
// Errors caused by a filesystem that does not support extended
// attributes are wrapped as ErrXattrUnsupported.
func SetXattr(nm string, x Xattr) error {
	for k, v := range x {
		if err := xattr.Set(nm, k, []byte(v)); err != nil {
			return wrapUnsupported(err)
		}
	}
	return nil
}

// LSetXattr sets/updates the xattr list for a given file.
// If 'nm' points to a symlink, LSetXattr will set/update the
// extended attributes of the symlink and *not* the target.
// Errors caused by a filesystem that does not support extended
// attributes are wrapped as ErrXattrUnsupported.
func LsetXattr(nm string, x Xattr) error {
	for k, v := range x {
		if err := xattr.LSet(nm, k, []byte(v)); err != nil {
			return wrapUnsupported(err)
		}
	}
	return nil
}

// ReplaceXattr replaces all the extended attributes of 'nm' with
// new attributes in 'x'. This function is a combination of
// ClearXattr() and SetXattr().
func ReplaceXattr(nm string, x Xattr) error {
	return repl(nm, x, xattr.List, xattr.Remove, xattr.Set)
}

// LReplaceXattr replaces all the extended attributes of 'nm' with
// new attributes in 'x'. This function is a combination of
// LclearXattr() and LsetXattr().
// If 'nm' points to a symlink, LReplaceXattr will set/update the
// extended attributes of the symlink and *not* the target.
func LreplaceXattr(nm string, x Xattr) error {
	return repl(nm, x, xattr.LList, xattr.LRemove, xattr.LSet)
}

// DelXattr deletes one or more extended attributes of a file.
// Errors from an xattr-less filesystem are wrapped as
// ErrXattrUnsupported.
func DelXattr(nm string, keys ...string) error {
	for _, k := range keys {
		if err := xattr.Remove(nm, k); err != nil {
			return wrapUnsupported(err)
		}
	}
	return nil
}

// LDelXattr deletes one or more extended attributes of a file.
// If 'nm' points to a symlink, LSetXattr will delete the
// extended attributes of the symlink and *not* the target.
// Errors from an xattr-less filesystem are wrapped as
// ErrXattrUnsupported.
func LdelXattr(nm string, keys ...string) error {
	for _, k := range keys {
		if err := xattr.LRemove(nm, k); err != nil {
			return wrapUnsupported(err)
		}
	}
	return nil
}

// ClearXattr deletes all the extended attributes of a file.
func ClearXattr(nm string) error {
	return clear(nm, xattr.List, xattr.Remove)
}

// ClearXattr deletes all the extended attributes of a file.
// If 'nm' points to a symlink, LSetXattr will delete the
// extended attributes of the symlink and *not* the target.
func LclearXattr(nm string) error {
	return clear(nm, xattr.LList, xattr.LRemove)
}

// handy helper that works for files and symlinks.
//
// Between the initial list() and each per-key get(), another process
// may remove an attribute. The kernel surfaces that as ENODATA on
// Linux or ENOATTR on darwin/BSD. Treat it as a benign race: skip the
// now-missing key and keep going, rather than aborting the whole fetch.
func fetch(nm string, list func(nm string) ([]string, error),
	get func(nm string, k string) ([]byte, error)) (Xattr, error) {
	keys, err := list(nm)
	if err != nil {
		return nil, wrapUnsupported(err)
	}

	x := make(Xattr)
	for _, k := range keys {
		b, err := get(nm, k)
		if err != nil {
			if isXattrNotFound(err) {
				continue
			}
			return nil, wrapUnsupported(err)
		}
		x[k] = string(b)
	}
	return x, nil
}

// handy helper to clear all xattr of nm; works for files and symlinks.
// Tolerates the same list/remove TOCTOU that fetch does.
func clear(nm string, list func(nm string) ([]string, error),
	del func(nm, key string) error) error {
	keys, err := list(nm)
	if err != nil {
		return wrapUnsupported(err)
	}

	for _, k := range keys {
		if err := del(nm, k); err != nil {
			if isXattrNotFound(err) {
				continue
			}
			return wrapUnsupported(err)
		}
	}
	return nil
}

// handy helper to replace all xattr of nm; works for files and symlinks
func repl(nm string, x Xattr, list func(nm string) ([]string, error),
	del func(nm, key string) error,
	set func(nm, key string, val []byte) error) error {

	if err := clear(nm, list, del); err != nil {
		return err
	}

	for k, v := range x {
		if err := set(nm, k, []byte(v)); err != nil {
			return wrapUnsupported(err)
		}
	}
	return nil
}
