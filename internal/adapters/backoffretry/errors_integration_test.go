// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/gostafa/singleton/internal/adapters/backoffretry"
	"github.com/gostafa/singleton/internal/domain"
)

var (
	errFactory = errors.New("factory failed")
	errStop    = errors.New("policy stopped")
)

func sameErrors(got, want []error) bool { return slices.EqualFunc(got, want, errors.Is) }

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

			initErr := backoffretry.NewInitError(domain.FailureExhausted, test.err, test.cause)

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

	initErr := backoffretry.NewInitError(domain.FailurePermanent, errFactory, errStop)

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

func TestPermanentReturnsNilForNil(t *testing.T) {
	t.Parallel()

	got := backoffretry.Permanent(nil)
	if got != nil {
		t.Errorf("Permanent(nil) = %v, want nil", got)
	}
}

func TestPermanentWrapsTheError(t *testing.T) {
	t.Parallel()

	got := backoffretry.Permanent(errFactory)

	var permanent *backoffretry.PermanentError

	if !errors.As(got, &permanent) {
		t.Fatalf("errors.As(%v, *PermanentError) = false, want true", got)
	}

	if !errors.Is(permanent.Err, errFactory) {
		t.Errorf("Err = %v, want %v", permanent.Err, errFactory)
	}

	if got.Error() != errFactory.Error() {
		t.Errorf("Error() = %q, want %q", got.Error(), errFactory.Error())
	}

	if !errors.Is(got, errFactory) {
		t.Errorf("errors.Is(got, errFactory) = false, want true")
	}
}
