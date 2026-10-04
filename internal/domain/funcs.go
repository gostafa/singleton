// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

import (
	"fmt"
)

// String returns a short human-readable description of r.
func (r FailureReason) String() string {
	names := [...]string{
		FailurePermanent: "permanent failure",
		FailureExhausted: "retries exhausted",
		FailureTimedOut:  "initialization timed out",
		FailureCanceled:  "initialization canceled",
	}

	if int(r) >= len(names) || names[r] == "" {
		return "initialization failed"
	}

	return names[r]
}

// NewInitError builds an InitError.
//
// cause is the retry policy's own stop condition rather than anything inferred
// from err. It is not exported on the result but stays reachable through
// [InitError.Unwrap], so no retry library leaks into the public API.
func NewInitError(reason FailureReason, err, cause error) *InitError {
	initErr := &InitError{
		Reason: reason,
		Err:    err,
		chain:  nil,
	}

	chain := make([]error, 0, chainCapacity)

	if err != nil {
		chain = append(chain, err)
	}

	if cause != nil {
		chain = append(chain, cause)
	}

	if len(chain) > 0 {
		initErr.chain = chain
	}

	return initErr
}

// Error formats the reason and the factory error as
// "singleton: <reason>: <err>".
func (e *InitError) Error() string {
	return fmt.Sprintf("singleton: %s: %v", e.Reason, e.Err)
}

// Unwrap returns the factory error together with the retry policy's stop
// condition, so errors.Is and errors.As match either.
//
// Because Unwrap returns a slice, the single-error errors.Unwrap returns nil
// for an InitError.
func (e *InitError) Unwrap() []error {
	if e.chain != nil {
		return e.chain
	}

	if e.Err != nil {
		return []error{e.Err}
	}

	return nil
}

// Error returns the wrapped error's message.
func (e *PermanentError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e *PermanentError) Unwrap() error {
	return e.Err
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

// Safe returns an observer that recovers from panics in o, or nil if o is nil.
//
// Instrumentation must never become the singleton's result. Without this, a
// panicking observer is caught by the recover that guards factory panics, and
// every later call for the life of the process re-panics with it. The
// composition root applies Safe before handing an observer to any adapter, so
// no adapter can violate the invariant.
func (o RetryObserver) Safe() RetryObserver {
	if o == nil {
		return nil
	}

	return func(event RetryEvent) {
		defer func() {
			_ = recover()
		}()

		o(event)
	}
}
