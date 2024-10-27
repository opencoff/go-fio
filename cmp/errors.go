// errors.go - descriptive errors for fio/cmp
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

package cmp

import (
	"fmt"
)

// Error represents the errors returned by
// CloneFile, CloneMetadata and UpdateMetadata
type Error struct {
	Op  string
	Src string
	Dst string
	Err error
}

// Error returns a string representation of Error
func (e *Error) Error() string {
	return fmt.Sprintf("cmp-tree: %s '%s' '%s': %s",
		e.Op, e.Src, e.Dst, e.Err.Error())
}

// Unwrap returns the underlying wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

var _ error = &Error{}
