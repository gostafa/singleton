// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/cenkalti/backoff/v7"
	"github.com/gostafa/singleton/internal/singleton/domain"
)

var errBoom = errors.New("boom")

func requireInitError(t *testing.T, err error) domain.InitError {
	t.Helper()

	var initErr domain.InitError

	if !errors.As(err, &initErr) {
		t.Fatalf("errors.As(%v, domain.InitError) = false, want true", err)
	}

	return initErr
}

func TestTranslateClassifiesByTheStopCondition(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want domain.FailureReason
	}{
		"not a retry error at all": {err: errBoom, want: domain.FailureExhausted},
		"permanent": {
			err:  &backoff.RetryError{LastErr: errBoom, Cause: backoff.ErrPermanent},
			want: domain.FailurePermanent,
		},
		"context deadline": {
			err:  &backoff.RetryError{LastErr: errBoom, Cause: context.DeadlineExceeded},
			want: domain.FailureTimedOut,
		},
		"maximum elapsed time": {
			err:  &backoff.RetryError{LastErr: errBoom, Cause: backoff.ErrMaxElapsedTime},
			want: domain.FailureTimedOut,
		},
		"context canceled": {
			err:  &backoff.RetryError{LastErr: errBoom, Cause: context.Canceled},
			want: domain.FailureCanceled,
		},
		"retries exhausted": {
			err:  &backoff.RetryError{LastErr: errBoom, Cause: backoff.ErrExhausted},
			want: domain.FailureExhausted,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			initErr := requireInitError(t, translate(test.err))
			if initErr.Reason() != test.want {
				t.Errorf("Reason = %v, want %v", initErr.Reason(), test.want)
			}

			if !errors.Is(initErr.Err(), errBoom) {
				t.Errorf("Err = %v, want %v", initErr.Err(), errBoom)
			}
		})
	}
}

var errFactory = errors.New("factory failed")

// sameErrors compares two unwrap chains positionally.
func sameErrors(got, want []error) bool {
	return slices.EqualFunc(got, want, errors.Is)
}

func TestInitErrorUnwrapFallsBackToErrWithoutAChain(t *testing.T) {
	t.Parallel()

	// newInitError always fills the chain, so a bare struct literal is the only
	// way to reach the fallback.
	initErr := &initError{details: failureDetails{reason: domain.FailureExhausted, err: errFactory}}

	if got := initErr.Unwrap(); !sameErrors(got, []error{errFactory}) {
		t.Errorf("Unwrap() = %v, want [%v]", got, errFactory)
	}

	empty := &initError{details: failureDetails{reason: domain.FailureExhausted}}

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

var errStop = errors.New("policy stopped")

func TestNewInitErrorBuildsTheUnwrapChain(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err   error
		cause error
		want  []error
	}{
		"factory error and stop condition": {
			err:   errFactory,
			cause: errStop,
			want:  []error{errFactory, errStop},
		},
		"factory error only":  {err: errFactory, cause: nil, want: []error{errFactory}},
		"stop condition only": {err: nil, cause: errStop, want: []error{errStop}},
		"neither":             {err: nil, cause: nil, want: nil},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			initErr := newInitError(domain.FailureExhausted, test.err, test.cause)

			if initErr.Reason() != domain.FailureExhausted {
				t.Errorf("Reason = %v, want %v", initErr.Reason(), domain.FailureExhausted)
			}

			if !errors.Is(initErr.Err(), test.err) {
				t.Errorf("Err = %v, want %v", initErr.Err(), test.err)
			}

			if got := initErr.Unwrap(); !sameErrors(got, test.want) {
				t.Errorf("Unwrap() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestInitErrorErrorFormatsReasonAndCause(t *testing.T) {
	t.Parallel()

	initErr := newInitError(domain.FailurePermanent, errFactory, errStop)

	const want = "singleton: permanent failure: factory failed"

	if got := initErr.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	if !errors.Is(initErr, errFactory) {
		t.Errorf("errors.Is(initErr, errFactory) = false, want true")
	}

	if !errors.Is(initErr, errStop) {
		t.Errorf("errors.Is(initErr, errStop) = false, want true")
	}

	// Unwrap returns a slice, so the single-error form finds nothing.
	unwrapped := errors.Unwrap(error(initErr))
	if unwrapped != nil {
		t.Errorf("errors.Unwrap(initErr) = %v, want nil", unwrapped)
	}
}
