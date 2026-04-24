// xattr_caps_linux_test.go -- unit tests for the Linux capability preflight.
//
// SPDX-License-Identifier: GPL-2.0
//
// (c) 2026 Sudhi Herle <sudhi@herle.net>
//
// Licensing Terms: GPLv2
//
// If you need a commercial license for this work, please contact
// the author.
//
// This software does not come with any express or implied
// warranty; it is provided "as is". No claim is made to its
// suitability for any purpose.

//go:build linux

package fio

import (
	"errors"
	"os"
	"testing"
)

// TestCapabilityFor verifies the xattr namespace → capability
// mapping. The three non-privileged namespaces (user, system,
// custom-app) must not require a capability; trusted and security
// must.
func TestCapabilityFor(t *testing.T) {
	cases := []struct {
		key        string
		wantCap    int
		wantNeeded bool
	}{
		{"user.foo", 0, false},
		{"system.posix_acl_access", 0, false},
		{"app.custom", 0, false},
		{"trusted.foo", capSysAdmin, true},
		{"trusted.", capSysAdmin, true},
		{"security.selinux", capMacAdmin, true},
		{"security.capability", capMacAdmin, true},
	}
	for _, tc := range cases {
		cap, needed := capabilityFor(tc.key)
		if cap != tc.wantCap || needed != tc.wantNeeded {
			t.Errorf("capabilityFor(%q) = (%d, %v); want (%d, %v)",
				tc.key, cap, needed, tc.wantCap, tc.wantNeeded)
		}
	}
}

// TestHasPrivilegedNamespaceKey confirms the fast-path predicate
// returns false for xattrs that contain no privileged-namespace
// keys (so we skip the /proc read in the common case).
func TestHasPrivilegedNamespaceKey(t *testing.T) {
	if hasPrivilegedNamespaceKey(Xattr{"user.a": "v", "user.b": "v"}) {
		t.Error("hasPrivilegedNamespaceKey(user-only) should be false")
	}
	if !hasPrivilegedNamespaceKey(Xattr{"user.a": "v", "trusted.x": "v"}) {
		t.Error("hasPrivilegedNamespaceKey(with trusted.*) should be true")
	}
	if !hasPrivilegedNamespaceKey(Xattr{"security.selinux": "v"}) {
		t.Error("hasPrivilegedNamespaceKey(with security.*) should be true")
	}
	if hasPrivilegedNamespaceKey(Xattr{}) {
		t.Error("hasPrivilegedNamespaceKey(empty) should be false")
	}
}

// TestCheckXattrCapabilitiesUserOnly verifies the preflight is a
// no-op (returns nil) when the only keys live in the user namespace,
// regardless of whether we're root or not.
func TestCheckXattrCapabilitiesUserOnly(t *testing.T) {
	if err := checkXattrCapabilities(Xattr{"user.foo": "v"}); err != nil {
		t.Errorf("checkXattrCapabilities(user.*) = %v, want nil", err)
	}
}

// TestCheckXattrCapabilitiesTrusted verifies the preflight's
// behavior depends on the process's actual capabilities. As root
// we expect success; as an unprivileged user we expect
// ErrXattrCapabilityMissing.
func TestCheckXattrCapabilitiesTrusted(t *testing.T) {
	err := checkXattrCapabilities(Xattr{"trusted.test.key": "v"})
	if os.Geteuid() == 0 {
		if err != nil {
			t.Errorf("as root: checkXattrCapabilities(trusted.*) = %v, want nil", err)
		}
		return
	}
	// non-root expectation
	if !errors.Is(err, ErrXattrCapabilityMissing) {
		t.Fatalf("non-root: expected ErrXattrCapabilityMissing, got %v", err)
	}
}

// TestHaveCapabilityRoot: if we're root, haveCapability should
// return true for any cap. If we're not, it returns false for an
// unknown/unheld cap. The sync.Once caching means we can't check
// both within one process, so this test only asserts the branch
// relevant to the current uid.
func TestHaveCapabilityCurrentProcess(t *testing.T) {
	if os.Geteuid() == 0 {
		if !haveCapability(capSysAdmin) {
			t.Error("as root: haveCapability(CAP_SYS_ADMIN) = false, want true")
		}
		return
	}
	// Unprivileged test runs almost never hold CAP_SYS_ADMIN.
	if haveCapability(capSysAdmin) {
		t.Skip("test process happens to hold CAP_SYS_ADMIN; skipping the unprivileged branch")
	}
}
