// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"time"

	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
)

type (
	// backOff is the delay policy used by the retry engine.
	backOff interface {
		// NextBackOff returns the next delay.
		NextBackOff() time.Duration
		// Reset starts a new delay sequence.
		Reset()
	}

	// InitError is an immutable classified initialization failure.
	InitError struct{ details failureDetails }

	failureDetails = struct {
		err    error
		chain  []error
		reason domain.FailureReason
	}

	// PermanentError marks a factory error as non-retriable.
	PermanentError struct {
		// Err is the non-retriable factory error.
		Err error
	}

	// Config tunes the retry policy.
	Config struct {
		// Observer receives each failed attempt that will be retried.
		Observer domain.RetryObserver
		// MaxAttempts includes the initial attempt.
		MaxAttempts uint
		// Timeout limits the complete initialization; zero disables it.
		Timeout time.Duration
		// InitialInterval is the first retry delay before jitter.
		InitialInterval time.Duration
		// MaxInterval caps the exponential delay before jitter.
		MaxInterval time.Duration
	}

	// Retrier runs an operation with exponential backoff and jitter.
	//
	// It implements [ports.Retrier].
	Retrier[T any] struct {
		cfg Config
	}
)
