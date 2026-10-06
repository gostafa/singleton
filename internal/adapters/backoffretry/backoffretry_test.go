// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"context"
	"errors"
	"testing"

	"github.com/cenkalti/backoff/v7"
	"github.com/gostafa/singleton/internal/domain"
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
