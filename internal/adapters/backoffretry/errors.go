// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"fmt"

	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
)

// NewInitError builds an InitError.
//
// cause is the retry policy's own stop condition rather than anything inferred
// from err. It is not exported on the result but stays reachable through
// [InitError.Unwrap], so no retry library leaks into the public API.
func NewInitError(reason domain.FailureReason, err, cause error) *InitError {
	return &InitError{
		details: failureDetails{reason: reason, err: err, chain: errorChain(err, cause)},
	}
}

// Error formats the reason and the factory error as
// "singleton: <reason>: <err>".
func (failure *InitError) Error() string {
	return fmt.Sprintf("singleton: %s: %v", reasonName(failure.details.reason), failure.details.err)
}

// Unwrap returns the factory error together with the retry policy's stop
// condition, so errors.Is and errors.As match either.
//
// Because Unwrap returns a slice, the single-error errors.Unwrap returns nil
// for an InitError.
func (failure *InitError) Unwrap() []error {
	return unwrapFailure(&failure.details)
}

// Err returns the final factory error.
func (failure *InitError) Err() error { return failure.details.err }

// Reason returns the policy's stop condition.
func (failure *InitError) Reason() domain.FailureReason { return failure.details.reason }

// Error returns the wrapped error's message.
func (failure *PermanentError) Error() string {
	return failure.Err.Error()
}

// Unwrap returns the wrapped error.
func (failure *PermanentError) Unwrap() error {
	return failure.Err
}

// Permanent marks a factory error as non-retriable.
//
// Permanent(nil) returns nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}

	return &PermanentError{Err: err}
}

// PermanentCause returns the non-retriable factory error.
func (failure *PermanentError) PermanentCause() error { return failure.Err }

func unwrapFailure(details *failureDetails) []error {
	if details.chain != nil {
		return details.chain
	}

	return errorChain(details.err, nil)
}

func reasonName(reason domain.FailureReason) string {
	names := [...]string{
		domain.FailurePermanent: "permanent failure",
		domain.FailureExhausted: "retries exhausted",
		domain.FailureTimedOut:  "initialization timed out",
		domain.FailureCanceled:  "initialization canceled",
	}
	if int(reason) >= len(names) || names[reason] == "" {
		return "initialization failed"
	}

	return names[reason]
}

func errorChain(err, cause error) []error {
	var chain []error

	if err != nil {
		chain = append(chain, err)
	}

	if cause != nil {
		chain = append(chain, cause)
	}

	return chain
}
