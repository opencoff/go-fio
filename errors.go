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

// ErrXattrUnsupported is returned (wrapped) from xattr operations
// when the target filesystem does not support extended attributes
// (the kernel reports ENOTSUP / EOPNOTSUPP). Callers that want to
// treat this as a non-fatal condition use errors.Is to detect it
// and downgrade to a skip - see clone.WithIgnoreUnsupported.
var ErrXattrUnsupported = errors.New("xattr: filesystem does not support extended attributes")

// ErrXattrCapabilityMissing is returned (wrapped) when a caller
// attempts to write an xattr whose namespace requires a Linux
// capability the process does not hold:
//
//   - trusted.*  requires CAP_SYS_ADMIN
//   - security.* requires CAP_MAC_ADMIN (or CAP_SYS_ADMIN depending on LSM)
//
// Surfacing this as a typed error lets callers emit a clear
// diagnostic ("needs CAP_SYS_ADMIN to restore trusted.xattr")
// instead of the raw EPERM that getxattr/setxattr would otherwise
// return. clone.WithIgnoreUnsupported also treats this error as
// skippable since the clone cannot succeed with the current
// privileges.
var ErrXattrCapabilityMissing = errors.New("xattr: required capability not held")

// ErrTooSmall is returned when a caller-supplied buffer cannot hold
// the encoded form of a value. Retained for backwards compatibility
// with the pre-proto marshaler's sentinel; callers should prefer
// errors.Is over string comparison.
var ErrTooSmall = errors.New("buffer is not big enough")
