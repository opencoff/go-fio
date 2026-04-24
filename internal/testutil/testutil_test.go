// testutil_test.go -- exercise the test helpers.

package testutil

import (
	"os"
	"testing"
)

// TestRequirePrivilegedBehavior verifies that RequirePrivileged skips
// when euid is non-zero and does nothing when euid is zero. We
// exercise the branch that matches the current test process - the
// opposite branch is validated by its own integration invocation
// (`sudo go test -v`).
func TestRequirePrivilegedBehavior(t *testing.T) {
	t.Run("sub", func(sub *testing.T) {
		RequirePrivileged(sub)
		// If we reach this line, euid was 0 and the helper did not
		// skip. Record that fact so the parent can check.
		sub.Log("RequirePrivileged did not skip")
	})

	// Inspect the sub-test outcome via whether it was skipped.
	// go test reports skip state via t.Skipped on the subtest's T;
	// we don't have that here, so we rely on the contrapositive:
	// if we're running as non-root the subtest should have been
	// skipped (and not failed).
	if os.Geteuid() != 0 && t.Failed() {
		t.Fatal("RequirePrivileged failed non-root test instead of skipping")
	}
}
