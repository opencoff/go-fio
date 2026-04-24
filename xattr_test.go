// xattr_test.go - tests for the xattr helpers that do not need a real
// filesystem.

package fio

import (
	"errors"
	"syscall"
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

// TestFetchSkipsNotFound verifies that an attribute disappearing
// between list() and get() (another process removing it) does not
// abort the whole fetch - the race is benign and the key is skipped.
func TestFetchSkipsNotFound(t *testing.T) {
	list := func(nm string) ([]string, error) {
		return []string{"user.a", "user.gone", "user.b"}, nil
	}

	get := func(nm, key string) ([]byte, error) {
		if key == "user.gone" {
			return nil, errXattrNotFound
		}
		return []byte("v-" + key), nil
	}

	x, err := fetch("/x", list, get)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if _, ok := x["user.gone"]; ok {
		t.Errorf("fetch: included vanished key user.gone")
	}
	if x["user.a"] != "v-user.a" || x["user.b"] != "v-user.b" {
		t.Errorf("fetch: surviving keys wrong: %v", x)
	}
}

// TestFetchPropagatesOtherGetError ensures non-ENODATA errors from
// get() still abort the fetch. A permission error on a privileged
// namespace must not be silently dropped.
func TestFetchPropagatesOtherGetError(t *testing.T) {
	errBoom := errors.New("get: boom")

	list := func(nm string) ([]string, error) {
		return []string{"user.a", "trusted.b"}, nil
	}
	get := func(nm, key string) ([]byte, error) {
		if key == "trusted.b" {
			return nil, errBoom
		}
		return []byte("ok"), nil
	}

	_, err := fetch("/x", list, get)
	if !errors.Is(err, errBoom) {
		t.Fatalf("fetch: expected err to wrap %v, got %v", errBoom, err)
	}
}

// TestClearSkipsNotFound verifies clear() swallows ENODATA from a
// del() that races with concurrent removal.
func TestClearSkipsNotFound(t *testing.T) {
	list := func(nm string) ([]string, error) {
		return []string{"user.a", "user.gone", "user.b"}, nil
	}

	var delKeys []string
	del := func(nm, key string) error {
		delKeys = append(delKeys, key)
		if key == "user.gone" {
			return errXattrNotFound
		}
		return nil
	}

	if err := clear("/x", list, del); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if len(delKeys) != 3 {
		t.Errorf("clear: del called %d times (want 3): %v", len(delKeys), delKeys)
	}
}

// TestClearPropagatesOtherDelError ensures non-ENODATA del errors
// still abort the clear.
func TestClearPropagatesOtherDelError(t *testing.T) {
	errBoom := errors.New("del: boom")
	list := func(nm string) ([]string, error) {
		return []string{"user.a", "user.b"}, nil
	}
	del := func(nm, key string) error {
		if key == "user.b" {
			return errBoom
		}
		return nil
	}
	if err := clear("/x", list, del); !errors.Is(err, errBoom) {
		t.Fatalf("clear: expected err wrapping %v, got %v", errBoom, err)
	}
}

// TestWrapUnsupportedENOTSUP verifies ENOTSUP is wrapped as
// ErrXattrUnsupported so callers can detect cross-FS clones onto
// filesystems without xattr support.
func TestWrapUnsupportedENOTSUP(t *testing.T) {
	got := wrapUnsupported(syscall.ENOTSUP)
	if got == nil {
		t.Fatalf("wrapUnsupported(ENOTSUP) = nil, want non-nil")
	}
	if !errors.Is(got, ErrXattrUnsupported) {
		t.Errorf("wrapUnsupported(ENOTSUP) not errors.Is ErrXattrUnsupported: %v", got)
	}
	if !errors.Is(got, syscall.ENOTSUP) {
		t.Errorf("wrapUnsupported(ENOTSUP) lost the wrapped errno: %v", got)
	}
}

// TestWrapUnsupportedEOPNOTSUPP is the same predicate for the
// POSIX-spelled twin (some kernels surface one vs the other).
func TestWrapUnsupportedEOPNOTSUPP(t *testing.T) {
	got := wrapUnsupported(syscall.EOPNOTSUPP)
	if !errors.Is(got, ErrXattrUnsupported) {
		t.Errorf("wrapUnsupported(EOPNOTSUPP) not errors.Is ErrXattrUnsupported: %v", got)
	}
}

// TestWrapUnsupportedPassthrough ensures unrelated errors are NOT
// tagged. A caller using errors.Is(err, ErrXattrUnsupported) must
// not false-positive on EACCES, EPERM, etc.
func TestWrapUnsupportedPassthrough(t *testing.T) {
	cases := []error{
		nil,
		syscall.EACCES,
		syscall.EPERM,
		syscall.EIO,
		errors.New("random"),
	}
	for _, in := range cases {
		got := wrapUnsupported(in)
		if errors.Is(got, ErrXattrUnsupported) {
			t.Errorf("wrapUnsupported(%v) wrongly tagged as ErrXattrUnsupported", in)
		}
	}
}

// TestFetchWrapsUnsupported verifies that an ENOTSUP from list() in
// the fetch path is surfaced as ErrXattrUnsupported.
func TestFetchWrapsUnsupported(t *testing.T) {
	list := func(nm string) ([]string, error) {
		return nil, syscall.ENOTSUP
	}
	get := func(nm, key string) ([]byte, error) {
		return nil, nil
	}

	_, err := fetch("/x", list, get)
	if !errors.Is(err, ErrXattrUnsupported) {
		t.Fatalf("fetch: expected ErrXattrUnsupported, got %v", err)
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
