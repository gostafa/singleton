// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton

import (
	"errors"

	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

var (
	// ErrNilFactory means New received a nil factory.
	ErrNilFactory = errors.New("singleton: factory is nil")

	// ErrInvalidOption means New received a zero-value Option.
	ErrInvalidOption = errors.New("singleton: invalid option")

	// ErrZeroMaxAttempts means the attempt budget is zero.
	ErrZeroMaxAttempts = errors.New("singleton: max attempts must be greater than zero")

	// ErrNegativeTimeout means the initialization timeout is negative.
	ErrNegativeTimeout = errors.New("singleton: initialization timeout cannot be negative")

	// ErrZeroInitialInterval means the initial retry interval is not positive.
	ErrZeroInitialInterval = errors.New(
		"singleton: initial retry interval must be greater than zero",
	)

	// ErrMaxIntervalBelowInitial means the maximum retry interval is less than the initial interval.
	ErrMaxIntervalBelowInitial = errors.New(
		"singleton: maximum retry interval cannot be less than the initial interval",
	)

	// ErrPermanent means initialization stopped because of a permanent factory error.
	ErrPermanent = errors.New("singleton: permanent failure")

	// ErrRetriesExhausted means initialization spent its attempt budget.
	ErrRetriesExhausted = errors.New("singleton: retries exhausted")

	_ ports.Retrier[int] = publicRetrier[int]{retrier: nil}

	_ Interface[int] = (*Provider[int])(nil)
)
