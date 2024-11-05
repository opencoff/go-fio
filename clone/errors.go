// errors.go - descriptive errors for fio/clone
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

package clone

import (
	"fmt"
	"strings"

	"github.com/opencoff/go-fio"
)

// Error represents the errors returned by
// CloneFile, CloneMetadata and UpdateMetadata
type Error struct {
	Op  string
	Src string
	Dst string
	Err error
}

// Error returns a string representation of a clone Error
func (e *Error) Error() string {
	return fmt.Sprintf("clonefile: %s '%s' '%s': %s",
		e.Op, e.Src, e.Dst, e.Err.Error())
}

// Unwrap returns the underlying wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

var _ error = &Error{}

// FunnyEntry captures an error where the source and destination
// are not the same type (eg src is a file and dst is a directory)
type FunnyEntry struct {
	Name string // relative path name
	Src  *fio.Info
	Dst  *fio.Info
}

// FunnyError represents a clone error that fails to clone a directory
// tree because there are one or more funny entries in the Src and Dst.
type FunnyError struct {
	Funny []FunnyEntry
}

// Error returns a string representation of FunnyError
func (e *FunnyError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "funny entries:\n")
	for i := range e.Funny {
		f := &e.Funny[i]
		fmt.Fprintf(&b, "\t%s:\n\t\t%s\n\t\t%s\n",
			f.Name, f.Src, f.Dst)
	}
	return b.String()
}

// Unwrap returns the underlying wrapped error; in our
// case this is a "leaf" error.
func (e *FunnyError) Unwrap() error {
	return nil
}

var _ error = &FunnyError{}
