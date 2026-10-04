// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

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
	Err    error
	chain  []error
	Reason FailureReason
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
	Err       error
	Attempt   uint
	NextDelay time.Duration
}

// RetryObserver receives one event per retried attempt.
type RetryObserver func(RetryEvent)
