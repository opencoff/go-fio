// errors.go - descriptive errors for fio
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

package fio

import (
	"errors"
	"fmt"
)

// errAny returns true if the target error 'err' matches
// any in the list 'errs'; and returns false otherwise
func errAny(err error, errs ...error) bool {
	for _, e := range errs {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// CopyError represents the errors returned by
// CopyFile and CopyFd
type CopyError struct {
	Op  string
	Src string
	Dst string
	Err error
}

// Error returns a string representation of CopyError
func (e *CopyError) Error() string {
	return fmt.Sprintf("copyfile: %s '%s' '%s': %s",
		e.Op, e.Src, e.Dst, e.Err.Error())
}

// Unwrap returns the underlying wrapped error
func (e *CopyError) Unwrap() error {
	return e.Err
}

var _ error = &CopyError{}
