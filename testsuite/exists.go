// exists.go -- handy checks for file/dir existence

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Return true if dir exists, false otherwise
// Wot a complicated way to do things in golang!
func DirExists(dn string) (bool, error) {
	st, err := os.Lstat(dn)
	if err == nil {
		if st.Mode().IsDir() {
			return true, nil
		}
		return false, fmt.Errorf("%s: entry exists but not a dir", dn)
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("DirExists: lstat %w", err)
}

// Return true if file exists, false otherwise
// Wot a complicated way to do things in golang!
func FileExists(dn string) (bool, error) {
	st, err := os.Lstat(dn)
	if err == nil {
		if st.Mode().IsRegular() {
			return true, nil
		}
		return false, fmt.Errorf("%s: entry exists but not a file", dn)
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("FileExists: lstat %w", err)
}
