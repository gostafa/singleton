// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"errors"
	"slices"
	"testing"

	"github.com/gostafa/singleton/internal/domain"
)

var errFactory = errors.New("factory failed")

// sameErrors compares two unwrap chains positionally.
func sameErrors(got, want []error) bool {
	return slices.EqualFunc(got, want, errors.Is)
}

func TestInitErrorUnwrapFallsBackToErrWithoutAChain(t *testing.T) {
	t.Parallel()

	// NewInitError always fills the chain, so a bare struct literal is the only
	// way to reach the fallback.
	initErr := &InitError{details: failureDetails{reason: domain.FailureExhausted, err: errFactory}}

	if got := initErr.Unwrap(); !sameErrors(got, []error{errFactory}) {
		t.Errorf("Unwrap() = %v, want [%v]", got, errFactory)
	}

	empty := &InitError{details: failureDetails{reason: domain.FailureExhausted}}

	if got := empty.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestReasonNameHandlesUnknownValues(t *testing.T) {
	t.Parallel()
	for _, reason := range []domain.FailureReason{0, 200} {
		if got := reasonName(reason); got != "initialization failed" {
			t.Errorf("reasonName(%d) = %q", reason, got)
		}
	}
}
