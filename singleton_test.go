// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton

import (
	"context"
	"errors"
	"testing"

	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
)

var errInternalStop = errors.New("private retry stop condition")

func TestPublicInitErrorReplacesInternalStopCauses(t *testing.T) {
	t.Parallel()

	tests := map[domain.FailureReason]struct {
		cause  error
		reason FailureReason
	}{
		domain.FailurePermanent: {reason: FailurePermanent, cause: ErrPermanent},
		domain.FailureExhausted: {reason: FailureExhausted, cause: ErrRetriesExhausted},
		domain.FailureTimedOut:  {reason: FailureTimedOut, cause: context.DeadlineExceeded},
		domain.FailureCanceled:  {reason: FailureCanceled, cause: context.Canceled},
	}

	for reason, test := range tests {
		t.Run(reason.String(), func(t *testing.T) {
			t.Parallel()

			internal := domain.NewInitError(reason, errBoom, errInternalStop)

			got := publicInitError(internal)
			if got.Reason != test.reason || !errors.Is(got, test.cause) {
				t.Errorf("public error = %v, unwrap = %v", got, got.Unwrap())
			}

			assertPublicErrorChain(t, got)
		})
	}
}

func assertPublicErrorChain(t *testing.T, got *InitError) {
	t.Helper()

	if !errors.Is(got.Err, errBoom) || !errors.Is(got, errBoom) {
		t.Error("public error lost the factory error")
	}

	if errors.Is(got, errInternalStop) {
		t.Error("public error leaked the internal stop cause")
	}

	if _, ok := errors.AsType[*domain.InitError](got); ok {
		t.Error("public error leaked the internal InitError")
	}
}

var errBoom = errors.New("boom")

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig().retry

	tests := map[string]struct{ got, want any }{
		"max attempts":     {got: cfg.MaxAttempts, want: uint(defaultMaxAttempts)},
		"timeout":          {got: cfg.Timeout, want: defaultTimeout},
		"initial interval": {got: cfg.InitialInterval, want: defaultInitialInterval},
		"maximum interval": {got: cfg.MaxInterval, want: defaultMaxInterval},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if test.got != test.want {
				t.Errorf("%s = %v, want %v", name, test.got, test.want)
			}
		})
	}
}

func TestDefaultConfigRegistersNoObserver(t *testing.T) {
	t.Parallel()

	if defaultConfig().retry.Observer != nil {
		t.Error("Observer = non-nil, want nil until WithRetryObserver registers one")
	}
}
