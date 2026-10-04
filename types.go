package singleton

import (
	"context"
	"github.com/mostafakhairy0305-dot/singleton/internal/adapters/backoffretry"
	"github.com/mostafakhairy0305-dot/singleton/internal/application"
	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
	"time"
)

// FailureReason explains why initialization stopped.
type FailureReason uint8

// InitError reports that shared initialization failed.
//
// Every initialization failure returned by Get is an *InitError. Caller-context
// failures are not *InitError values.
//
// Classify a failure with Reason, not with errors.Is: a factory that fails with
// context.DeadlineExceeded on every attempt exhausts its retry budget, so
// Reason is FailureExhausted even though errors.Is matches context.DeadlineExceeded.
type InitError struct {
	// Reason reports why initialization stopped. It is the authoritative classification.
	Reason FailureReason

	// Err is the last error the factory returned.
	Err error

	chain []error
}

// PermanentError marks a factory error as non-retriable.
// Build one with Permanent or return a *PermanentError from a factory.
type PermanentError struct {
	// Err is the wrapped, non-retriable error.
	Err error
}

// Option configures a [Provider].
//
// Its implementation is unexported so callers cannot depend on the retry
// library underneath. The zero value is invalid and is rejected by [New].
type Option struct {
	apply func(*config) error
}

type config struct {
	retry backoffretry.Config
}

// RetryEvent describes a failed attempt that will be retried. It is delivered
// to the observer registered with WithRetryObserver.
type RetryEvent struct {
	// Attempt is the 1-based number of the attempt that just failed.
	Attempt uint

	// Err is why that attempt failed.
	Err error

	// NextDelay is how long the policy waits before the next attempt.
	NextDelay time.Duration
}

// publicRetrier converts API values at the boundary, before the application
// caches a result. Each failed initialization has one shared public error.
type publicRetrier[T any] struct {
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
type Factory[T any] = func(context.Context) (T, error)

// Provider lazily initializes and returns one shared value.
//
// Create one with [New] or [MustNew]. The zero value is not usable and a
// Provider must not be copied after first use. It is safe for concurrent use.
type Provider[T any] struct {
	provider *application.Provider[T]
}

// Interface is the behaviour [Provider] implements.
//
// Depend on it in consumers that need to substitute a fake.
type Interface[T any] interface {
	// Get waits for and returns the shared value.
	Get(ctx context.Context) (T, error)

	// Reset discards a failed initialization so the next Get starts a new one.
	Reset()
}
