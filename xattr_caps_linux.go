// xattr_caps_linux.go -- Linux capability preflight for xattr writes.
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
// warranty; it is provided "as is". No claim  is made to its
// suitability for any purpose.

//go:build linux

package fio

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Linux capability numbers used by the xattr preflight. Values are
// stable uapi constants; we keep the short list here rather than
// depending on x/sys/unix for just two names.
const (
	capSysAdmin = 21
	capMacAdmin = 33
)

// capName maps capability numbers to human-readable names for error
// messages.
var capName = map[int]string{
	capSysAdmin: "CAP_SYS_ADMIN",
	capMacAdmin: "CAP_MAC_ADMIN",
}

// capabilityFor reports which Linux capability (if any) is required
// to write an xattr with the given key name. Keys not in a
// privileged namespace return (0, false).
//
// Namespaces:
//   - user.*     — any process that owns the file (or CAP_FOWNER)
//   - trusted.*  — CAP_SYS_ADMIN
//   - security.* — CAP_MAC_ADMIN (or CAP_SYS_ADMIN depending on LSM)
//   - system.*   — kernel-interpreted (POSIX ACLs); ownership-gated
//
// For security.* the preflight is over-conservative: an SELinux
// policy may permit a specific process to relabel without
// CAP_MAC_ADMIN. Accepting the false-positive is better than a
// cryptic EPERM at restore time; callers can override with
// clone.WithIgnoreUnsupported to demote the error to a skip.
func capabilityFor(key string) (cap int, needed bool) {
	switch {
	case strings.HasPrefix(key, "trusted."):
		return capSysAdmin, true
	case strings.HasPrefix(key, "security."):
		return capMacAdmin, true
	}
	return 0, false
}

// checkXattrCapabilities scans an Xattr map for keys whose
// namespace requires a capability the process does not hold.
// Returns the first offending key wrapped in ErrXattrCapabilityMissing,
// or nil if all keys are writable.
//
// Fast path: iterate only if at least one key looks privileged;
// otherwise skip the /proc read entirely.
func checkXattrCapabilities(x Xattr) error {
	if !hasPrivilegedNamespaceKey(x) {
		return nil
	}
	for k := range x {
		cap, needed := capabilityFor(k)
		if !needed {
			continue
		}
		if haveCapability(cap) {
			continue
		}
		return fmt.Errorf("%w: %s needed to set %q", ErrXattrCapabilityMissing, capName[cap], k)
	}
	return nil
}

// hasPrivilegedNamespaceKey reports whether any key in x belongs
// to a namespace that requires elevated capabilities.
func hasPrivilegedNamespaceKey(x Xattr) bool {
	for k := range x {
		if _, needed := capabilityFor(k); needed {
			return true
		}
	}
	return false
}

// cached effective-capability mask. /proc/self/status cannot change
// during a process's lifetime without explicit syscalls (setuid,
// prctl, etc.), and nothing in this package does that — so a one-
// shot read is safe.
var (
	capOnce sync.Once
	capEff  uint64
	capErr  error
)

// haveCapability reports whether the current process has the given
// Linux capability in its effective set. euid==0 is a fast-path
// yes. A process that cannot open /proc/self/status gets a
// conservative "no".
func haveCapability(cap int) bool {
	if os.Geteuid() == 0 {
		return true
	}
	capOnce.Do(func() {
		capEff, capErr = readCapEff()
	})
	if capErr != nil {
		return false
	}
	return capEff&(1<<uint(cap)) != 0
}

// readCapEff parses /proc/self/status for the CapEff line and
// returns its hex-decoded value. We read the effective set (not
// bounding) because that is what the kernel checks against a
// syscall at call time.
func readCapEff() (uint64, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		const prefix = "CapEff:"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		hex := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		n, err := strconv.ParseUint(hex, 16, 64)
		if err != nil {
			return 0, fmt.Errorf("parse CapEff %q: %w", hex, err)
		}
		return n, nil
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	return 0, errors.New("CapEff line not found in /proc/self/status")
}
