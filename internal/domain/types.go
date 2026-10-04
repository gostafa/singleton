package domain

import (
	"time"
)

// FailureReason explains why initialization stopped.
type FailureReason uint8

// InitError reports that shared initialization failed.
//
// Classify a failure with Reason, not with errors.Is. [InitError.Unwrap]
// deliberately exposes both the factory error and the reason retrying stopped,
// so errors.Is answers whether an error appears anywhere in the chain — a
// different question from why initialization stopped. A factory that fails
// with context.DeadlineExceeded on every attempt exhausts its retry budget, so
// Reason is [FailureExhausted] even though errors.Is reports a match for
// context.DeadlineExceeded.
//
// Build one with [NewInitError].
type InitError struct {
	// Reason reports why initialization stopped. It is the authoritative
	// classification.
	Reason FailureReason

	// Err is the last error the factory returned.
	Err error

	chain []error
}

// PermanentError marks a factory error as non-retriable.
//
// A retry adapter detects it with errors.As and stops immediately.
type PermanentError struct {
	// Err is the wrapped, non-retriable error.
	Err error
}

// RetryEvent describes a failed attempt that will be retried.
type RetryEvent struct {
	// Attempt is the 1-based number of the attempt that just failed.
	Attempt uint

	// Err is why that attempt failed.
	Err error

	// NextDelay is how long the policy waits before the next attempt.
	NextDelay time.Duration
}

// RetryObserver receives one event per retried attempt.
type RetryObserver func(RetryEvent)
