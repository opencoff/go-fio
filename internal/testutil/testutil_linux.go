// testutil_linux.go -- linux-only test helpers.
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

package testutil

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Linux capability numbers relevant to the go-fio privileged tests.
// Values are stable (defined by uapi/linux/capability.h) but we keep
// the short list here instead of pulling in x/sys/unix's full set.
const (
	CAP_FOWNER     = 3
	CAP_SYS_ADMIN  = 21
	CAP_MKNOD      = 27
	CAP_MAC_ADMIN  = 33
)

// capName maps a capability number back to its symbolic name for
// skip messages. Extend as needed.
var capName = map[int]string{
	CAP_FOWNER:    "CAP_FOWNER",
	CAP_SYS_ADMIN: "CAP_SYS_ADMIN",
	CAP_MKNOD:     "CAP_MKNOD",
	CAP_MAC_ADMIN: "CAP_MAC_ADMIN",
}

// RequireCapability skips the test unless the calling process has
// the given Linux capability in its *effective* set. Parses
// /proc/self/status (specifically the "CapEff:" line) because the
// alternatives - Prctl(PR_CAPBSET_READ, ...) or Capget(2) - inspect
// the bounding/inheritable set, not the effective set.
//
// If euid is already 0 we short-circuit with a "yes" answer; the
// effective capability set of a full-root process includes
// everything.
func RequireCapability(t *testing.T, cap int) {
	t.Helper()
	if os.Geteuid() == 0 {
		return
	}
	eff, err := readCapEff()
	if err != nil {
		t.Skipf("%s: cannot read /proc/self/status to verify capability: %v", t.Name(), err)
	}
	if eff&(1<<cap) == 0 {
		name, ok := capName[cap]
		if !ok {
			name = fmt.Sprintf("cap#%d", cap)
		}
		t.Skipf("needs %s (or root) to test %s; skipping", name, t.Name())
	}
}

// readCapEff returns the effective capability bitmask of the calling
// process by parsing /proc/self/status. The CapEff line encodes the
// mask as a 16-digit hex string.
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
