// ignore_unsupported_test.go -- verifies the metaOpt.ignoreUnsupported
// flag causes updateMeta to skip cloners that return an error wrapping
// fio.ErrXattrUnsupported, while still propagating other errors and
// running the remaining cloners.
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

package clone

import (
	"errors"
	"fmt"
	"testing"

	"github.com/opencoff/go-fio"
)

// TestUpdateMetaIgnoresUnsupported replaces the package-level
// mdUpdaters pipeline with a handful of instrumented cloners:
//   - a "pre" step that always succeeds and is expected to run
//   - an "xattr" step that returns an ErrXattrUnsupported-wrapped error
//   - a "post" step that MUST still run when the flag is set
//
// The test first runs with opt.ignoreUnsupported=false and expects
// the xattr error to surface and "post" to NOT run. Then it runs
// with the flag set and expects the call to succeed with "post" having
// executed.
func TestUpdateMetaIgnoresUnsupported(t *testing.T) {
	var preRan, postRan int

	// swap mdUpdaters for the duration of the test
	orig := mdUpdaters
	t.Cleanup(func() { mdUpdaters = orig })

	xattrErr := fmt.Errorf("set xattr: %w", fio.ErrXattrUnsupported)

	mdUpdaters = []cloner{
		func(_ string, _ *fio.Info) error { preRan++; return nil },
		func(_ string, _ *fio.Info) error { return xattrErr },
		func(_ string, _ *fio.Info) error { postRan++; return nil },
	}

	fi := &fio.Info{}
	fi.SetPath("/fake")

	// strict mode: error bubbles, post does not run
	preRan, postRan = 0, 0
	if err := updateMeta("/fake-dst", fi, metaOpt{}); err == nil {
		t.Fatalf("updateMeta strict: expected error, got nil")
	} else if !errors.Is(err, fio.ErrXattrUnsupported) {
		t.Errorf("updateMeta strict: got %v, want wrapped ErrXattrUnsupported", err)
	}
	if preRan != 1 {
		t.Errorf("strict: pre ran %d times, want 1", preRan)
	}
	if postRan != 0 {
		t.Errorf("strict: post ran %d times, want 0", postRan)
	}

	// ignore mode: error is swallowed, post runs
	preRan, postRan = 0, 0
	if err := updateMeta("/fake-dst", fi, metaOpt{ignoreUnsupported: true}); err != nil {
		t.Fatalf("updateMeta ignore: unexpected error %v", err)
	}
	if preRan != 1 {
		t.Errorf("ignore: pre ran %d times, want 1", preRan)
	}
	if postRan != 1 {
		t.Errorf("ignore: post ran %d times, want 1 (ignoreUnsupported did not skip)", postRan)
	}
}

// TestUpdateMetaIgnoresCapabilityMissing is the capability-side
// parallel of TestUpdateMetaIgnoresUnsupported. If the xattr cloner
// fails with ErrXattrCapabilityMissing (e.g. non-root trying to
// restore a trusted.* xattr), the ignoreUnsupported flag must also
// skip it so the remaining cloners still run.
func TestUpdateMetaIgnoresCapabilityMissing(t *testing.T) {
	orig := mdUpdaters
	t.Cleanup(func() { mdUpdaters = orig })

	capErr := fmt.Errorf("no CAP_SYS_ADMIN: %w", fio.ErrXattrCapabilityMissing)

	var postRan int
	mdUpdaters = []cloner{
		func(_ string, _ *fio.Info) error { return capErr },
		func(_ string, _ *fio.Info) error { postRan++; return nil },
	}

	fi := &fio.Info{}
	fi.SetPath("/fake")

	// strict mode: error bubbles
	postRan = 0
	if err := updateMeta("/fake-dst", fi, metaOpt{}); err == nil {
		t.Fatalf("strict: expected error, got nil")
	} else if !errors.Is(err, fio.ErrXattrCapabilityMissing) {
		t.Errorf("strict: got %v, want wrapped ErrXattrCapabilityMissing", err)
	}
	if postRan != 0 {
		t.Errorf("strict: post ran %d, want 0", postRan)
	}

	// ignore mode: capability error is swallowed, post runs
	postRan = 0
	if err := updateMeta("/fake-dst", fi, metaOpt{ignoreUnsupported: true}); err != nil {
		t.Fatalf("ignore: unexpected error %v", err)
	}
	if postRan != 1 {
		t.Errorf("ignore: post ran %d, want 1", postRan)
	}
}

// TestUpdateMetaNonUnsupportedStillFails confirms the flag is narrow:
// a non-ErrXattrUnsupported error in the xattr slot still fails the
// clone even when ignoreUnsupported is set.
func TestUpdateMetaNonUnsupportedStillFails(t *testing.T) {
	orig := mdUpdaters
	t.Cleanup(func() { mdUpdaters = orig })

	other := errors.New("generic clone failure")
	mdUpdaters = []cloner{
		func(_ string, _ *fio.Info) error { return other },
	}

	fi := &fio.Info{}
	fi.SetPath("/fake")

	err := updateMeta("/fake-dst", fi, metaOpt{ignoreUnsupported: true})
	if err == nil {
		t.Fatalf("updateMeta: expected error to propagate, got nil")
	}
	if !errors.Is(err, other) {
		t.Errorf("updateMeta: got %v, want wrapping %v", err, other)
	}
}
