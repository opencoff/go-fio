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
	"fmt"
	"strings"

	"github.com/pkg/xattr"
)

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

	for y, _ := range x {
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
func SetXattr(nm string, x Xattr) error {
	for k, v := range x {
		if err := xattr.Set(nm, k, []byte(v)); err != nil {
			return err
		}
	}
	return nil
}

// LSetXattr sets/updates the xattr list for a given file.
// If 'nm' points to a symlink, LSetXattr will set/update the
// extended attributes of the symlink and *not* the target.
func LsetXattr(nm string, x Xattr) error {
	for k, v := range x {
		if err := xattr.LSet(nm, k, []byte(v)); err != nil {
			return err
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
func DelXattr(nm string, keys ...string) error {
	for _, k := range keys {
		if err := xattr.Remove(nm, k); err != nil {
			return err
		}
	}
	return nil
}

// LDelXattr deletes one or more extended attributes of a file.
// If 'nm' points to a symlink, LSetXattr will delete the
// extended attributes of the symlink and *not* the target.
func LdelXattr(nm string, keys ...string) error {
	for _, k := range keys {
		if err := xattr.LRemove(nm, k); err != nil {
			return err
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

// handy helper that works for files and symlinks
func fetch(nm string, list func(nm string) ([]string, error),
	get func(nm string, k string) ([]byte, error)) (Xattr, error) {
	keys, err := list(nm)
	if err != nil {
		return nil, err
	}

	x := make(Xattr)
	for _, k := range keys {
		b, err := get(nm, k)
		if err != nil {
			return nil, err
		}
		x[k] = string(b)
	}
	return x, nil
}

// handy helper to clear all xattr of nm; works for files and symlinks
func clear(nm string, list func(nm string) ([]string, error),
	del func(nm, key string) error) error {
	keys, err := list(nm)
	if err != nil {
		return err
	}

	for _, k := range keys {
		if err = del(nm, k); err != nil {
			return err
		}
	}
	return err
}

// handy helper to replace all xattr of nm; works for files and symlinks
func repl(nm string, x Xattr, list func(nm string) ([]string, error),
	del func(nm, key string) error,
	set func(nm, key string, val []byte) error) error {

	if err := clear(nm, list, del); err != nil {
		return nil
	}

	for k, v := range x {
		if err := set(nm, k, []byte(v)); err != nil {
			return err
		}
	}
	return nil
}
