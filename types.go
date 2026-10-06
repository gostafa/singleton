// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// revive:disable:max-public-structs The public API requires these five distinct value types.

package singleton

import (
	"context"
	"time"

	"github.com/gostafa/singleton/internal/adapters/backoffretry"
	"github.com/gostafa/singleton/internal/application"
	"github.com/gostafa/singleton/internal/ports"
)

type (
	// FailureReason explains why initialization stopped.
	FailureReason uint8

	// InitError reports that shared initialization failed.
	//
	// Every initialization failure returned by Get is an *InitError. Caller-context
	// failures are not *InitError values.
	//
	// Classify a failure with Reason(), not with errors.Is: a factory that fails with
	// context.DeadlineExceeded on every attempt exhausts its retry budget, so
	// Reason() is FailureExhausted even though errors.Is matches context.DeadlineExceeded.
	// Construct a standalone failure with [NewInitError]; read its cause with [InitError.Err].
	InitError struct{ details failureDetails }

	failureDetails = struct {
		err    error
		chain  []error
		reason FailureReason
	}

	// PermanentError marks a factory error as non-retriable.
	// Build one with Permanent or return a *PermanentError from a factory.
	PermanentError struct {
		// Err is the wrapped, non-retriable error.
		Err error
	}

	// Option configures a [Provider].
	//
	// Its implementation is unexported so callers cannot depend on the retry
	// library underneath. The zero value is invalid and is rejected by [New].
	Option struct {
		apply func(*config) error
	}

	config = struct {
		retry backoffretry.Config
	}

	// RetryEvent describes a failed attempt that will be retried. It is delivered
	// to the observer registered with WithRetryObserver.
	RetryEvent struct {
		// Err is the error returned by this attempt.
		Err error
		// Attempt is the number of the failed attempt, starting at one.
		Attempt uint
		// NextDelay is the wait before the next attempt.
		NextDelay time.Duration
	}

	// publicRetrier converts API values at the boundary, before the application
	// caches a result. Each failed initialization has one shared public error.
	publicRetrier[T any] struct {
		retrier ports.Retrier[T]
	}

	// Factory creates the singleton value.
	//
	// The context belongs to this package: it carries the deadline set by
	// [WithInitializationTimeout], not any caller's deadline.
	//
	// A factory that returns an error must release whatever it has already
	// acquired. Every failed attempt's return value is discarded, so a factory that
	// dials a connection and then fails validation leaks one connection per
	// attempt. Wrap the error with [Permanent] when retrying cannot help.
	//
	// Factory is an alias for func(context.Context) (T, error).
	Factory[T any] = func(context.Context) (T, error)

	// Provider lazily initializes and returns one shared value.
	//
	// Create one with [New] or [MustNew]. The zero value is not usable and a
	// Provider must not be copied after first use. It is safe for concurrent use.
	Provider[T any] struct {
		provider application.Interface[T]
	}

	// Interface is the behavior [Provider] implements.
	//
	// Depend on it in consumers that need to substitute a fake.
	Interface[T any] interface {
		// Get waits for and returns the shared value.
		Get(ctx context.Context) (T, error)

		// Reset discards a failed initialization so the next Get starts a new one.
		Reset()
	}
)
