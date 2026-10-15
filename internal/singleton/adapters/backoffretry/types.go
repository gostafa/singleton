// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package backoffretry implements the retry port with exponential backoff and
// jitter. Concrete retry policies and failure types stay private to the adapter.
package backoffretry

import (
	"time"

	"github.com/gostafa/singleton/internal/singleton/domain"
)

type (
	// backOff is the delay policy used by the retry engine.
	backOff interface {
		// NextBackOff returns the next delay.
		NextBackOff() time.Duration
		// Reset starts a new delay sequence.
		Reset()
	}

	// initError is an immutable classified initialization failure.
	initError struct{ details failureDetails }

	failureDetails = struct {
		err    error
		chain  []error
		reason domain.FailureReason
	}

	// permanentError marks a factory error as non-retriable.
	permanentError struct {
		err error
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

	// retrier runs an operation with exponential backoff and jitter.
	//
	// It implements [ports.Retrier].
	retrier[T any] struct {
		cfg Config
	}
)
