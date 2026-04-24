// xattr_filter_darwin_test.go -- darwin-only tests for the
// com.apple.system.Security xattr filter. Covers the predicate
// directly, the filterKeys helper, and end-to-end propagation
// through fetch / clear / SetXattr via stub callbacks.

//go:build darwin

package fio

import (
	"reflect"
	"testing"
)

// TestIsFilteredXattr confirms only the reserved darwin ACL key is
// filtered. Other com.apple.* keys are application metadata and
// round-trip fine via setxattr.
func TestIsFilteredXattr(t *testing.T) {
	cases := map[string]bool{
		darwinACLKey:               true,
		"com.apple.FinderInfo":     false,
		"com.apple.quarantine":     false,
		"com.apple.metadata:_kMDI": false,
		"user.foo":                 false,
		"":                         false,
	}
	for k, want := range cases {
		if got := isFilteredXattr(k); got != want {
			t.Errorf("isFilteredXattr(%q) = %v, want %v", k, got, want)
		}
	}
}

// TestFilterKeys verifies the in-place compaction drops only the
// reserved key.
func TestFilterKeys(t *testing.T) {
	in := []string{
		"user.a",
		darwinACLKey,
		"com.apple.FinderInfo",
		darwinACLKey,
		"user.b",
	}
	want := []string{"user.a", "com.apple.FinderInfo", "user.b"}

	got := filterKeys(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterKeys: got %v, want %v", got, want)
	}
}

// TestFetchFiltersDarwinACL injects a list() that includes
// darwinACLKey and verifies fetch skips it without calling get()
// for that key.
func TestFetchFiltersDarwinACL(t *testing.T) {
	list := func(nm string) ([]string, error) {
		return []string{"user.a", darwinACLKey, "user.b"}, nil
	}

	var gotCalls []string
	get := func(nm, key string) ([]byte, error) {
		gotCalls = append(gotCalls, key)
		return []byte("v-" + key), nil
	}

	x, err := fetch("/x", list, get)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, ok := x[darwinACLKey]; ok {
		t.Errorf("fetch returned %q in result map", darwinACLKey)
	}
	for _, c := range gotCalls {
		if c == darwinACLKey {
			t.Errorf("get called for filtered key %q", darwinACLKey)
		}
	}
}

// TestClearFiltersDarwinACL verifies clear skips del() for the
// reserved key so we never try to removexattr an unremovable
// attribute.
func TestClearFiltersDarwinACL(t *testing.T) {
	list := func(nm string) ([]string, error) {
		return []string{"user.a", darwinACLKey, "user.b"}, nil
	}
	var delKeys []string
	del := func(nm, key string) error {
		delKeys = append(delKeys, key)
		return nil
	}
	if err := clear("/x", list, del); err != nil {
		t.Fatalf("clear: %v", err)
	}
	for _, k := range delKeys {
		if k == darwinACLKey {
			t.Errorf("del called for filtered key %q", darwinACLKey)
		}
	}
}
