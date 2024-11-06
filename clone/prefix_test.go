// prefix_test.go -- testcases for longestPrefix
//
// (c) 2024- Sudhi Herle <sudhi@herle.net>
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
	"slices"
	"testing"
)

func TestPrefixSingleton(t *testing.T) {
	assert := newAsserter(t)
	dir := []string{"a"}

	dir = longestPrefixes(dir)
	assert(len(dir) == 1, "n-count: exp 1, saw %d", len(dir))
	assert(dir[0] == "a", "item wrong: exp %q, saw %q", "a", dir[0])
}

func TestPrefixMany(t *testing.T) {
	assert := newAsserter(t)
	dirs := []string{
		"a/b",
		"a/c",
		"a/b/c/d",
		"z/f",
		"b/c",
		"b/d",
		"a/b/c",
		"a",
	}
	exp := []string{"a/b/c/d", "a/c", "b/c", "b/d", "z/f"}

	dirs = longestPrefixes(dirs)
	assert(slices.Equal(dirs, exp), "slices unequal:\nexp %v\nsaw %v", exp, dirs)
}

func TestPrefixNone(t *testing.T) {
	assert := newAsserter(t)
	dirs := []string{
		"c/d",
		"e/f",
		"a/b",
	}
	exp := []string{"a/b", "c/d", "e/f"}

	dirs = longestPrefixes(dirs)
	assert(slices.Equal(dirs, exp), "slices unequal:\nexp %v\nsaw %v", exp, dirs)
}

func TestPrefixAll(t *testing.T) {
	assert := newAsserter(t)
	dirs := []string{
		"a/b/c/d",
		"a/b/c",
		"a/b",
		"a",
	}
	exp := []string{"a/b/c/d"}

	dirs = longestPrefixes(dirs)
	assert(slices.Equal(dirs, exp), "slices unequal:\nexp %v\nsaw %v", exp, dirs)
}
