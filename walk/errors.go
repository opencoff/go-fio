// errors.go - descriptive errors for fio/walk
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

package walk

import (
	"fmt"
)

// Error represents the errors returned by
// CloneFile, CloneMetadata and UpdateMetadata
type Error struct {
	Op   string
	Name string
	Err  error
}

// Error returns a string representation of Error
func (e *Error) Error() string {
	return fmt.Sprintf("walk: %s '%s': %s",
		e.Op, e.Name, e.Err.Error())
}

// Unwrap returns the underlying wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

var _ error = &Error{}
