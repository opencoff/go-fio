// xattr_test.go - tests for the xattr helpers that do not need a real
// filesystem.

package fio

import (
	"errors"
	"testing"
)

// TestReplClearErrorPropagates verifies that when the clear() step of
// repl() fails, the error is propagated to the caller rather than
// silently swallowed.
func TestReplClearErrorPropagates(t *testing.T) {
	errBoom := errors.New("del: boom")

	list := func(nm string) ([]string, error) {
		return []string{"user.a", "user.b"}, nil
	}

	// del fails on the second key; clear() should return the error and
	// repl() must propagate it.
	del := func(nm, key string) error {
		if key == "user.b" {
			return errBoom
		}
		return nil
	}

	setCalls := 0
	set := func(nm, key string, val []byte) error {
		setCalls++
		return nil
	}

	x := Xattr{"user.new": "v"}
	err := repl("/does/not/matter", x, list, del, set)
	if err == nil {
		t.Fatalf("repl: expected error when clear() fails, got nil")
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("repl: expected err to wrap %v, got %v", errBoom, err)
	}
	if setCalls != 0 {
		t.Fatalf("repl: set() must not be invoked after clear() failure; called %d times", setCalls)
	}
}

// TestReplClearSuccessRunsSet verifies the happy path: when clear()
// succeeds, repl() proceeds to set the new attributes.
func TestReplClearSuccessRunsSet(t *testing.T) {
	list := func(nm string) ([]string, error) {
		return []string{"user.old"}, nil
	}

	delCalls := 0
	del := func(nm, key string) error {
		delCalls++
		return nil
	}

	gotKeys := map[string]string{}
	set := func(nm, key string, val []byte) error {
		gotKeys[key] = string(val)
		return nil
	}

	x := Xattr{"user.a": "1", "user.b": "2"}
	if err := repl("/does/not/matter", x, list, del, set); err != nil {
		t.Fatalf("repl: unexpected error: %v", err)
	}
	if delCalls != 1 {
		t.Fatalf("repl: expected del() called 1 time for %q, got %d", "user.old", delCalls)
	}
	if gotKeys["user.a"] != "1" || gotKeys["user.b"] != "2" {
		t.Fatalf("repl: set() did not receive expected keys; got %v", gotKeys)
	}
}
