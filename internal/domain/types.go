// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"time"
)

type (
	// FailureReason identifies the retry policy's stop condition.
	FailureReason = uint8

	// InitError exposes a classified initialization failure.
	InitError interface {
		error
		// Err returns the final factory error.
		Err() error
		// Reason returns the policy's stop condition.
		Reason() FailureReason
		// Unwrap exposes the factory error and stop condition.
		Unwrap() []error
	}

	// PermanentError marks an error that must not be retried.
	PermanentError interface {
		error
		// PermanentCause returns the non-retriable factory error.
		PermanentCause() error
	}

	// RetryEvent is the data delivered between attempts.
	RetryEvent = struct {
		// Err is the failed attempt's error.
		Err error
		// Attempt counts attempts starting at one.
		Attempt uint
		// NextDelay is the wait before retrying.
		NextDelay time.Duration
	}

	// RetryObserver receives one event per retried attempt.
	RetryObserver = func(RetryEvent)
)
