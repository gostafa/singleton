package domain

import (
	"errors"
	"slices"
	"testing"
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
	initErr := &InitError{Reason: FailureExhausted, Err: errFactory, chain: nil}

	if got := initErr.Unwrap(); !sameErrors(got, []error{errFactory}) {
		t.Errorf("Unwrap() = %v, want [%v]", got, errFactory)
	}

	empty := &InitError{Reason: FailureExhausted, Err: nil, chain: nil}

	if got := empty.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}
