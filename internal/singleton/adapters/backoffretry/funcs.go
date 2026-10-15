// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/gostafa/singleton/internal/singleton/domain"
	"github.com/gostafa/singleton/internal/singleton/ports"
)

// New builds a retry adapter for values of type Value, exposed through its port.
//
//nolint:gocritic // Copy configuration so later caller mutations cannot change the policy.
func New[Value any](cfg Config) ports.Retrier[Value] {
	return &retrier[Value]{cfg: cfg}
}

// Do runs op until it succeeds or the policy stops.
//
// On failure it returns the zero Value and a [domain.InitError] whose Reason comes
// from the policy's own stop condition rather than from the operation's error.
func (retrier *retrier[Value]) Do(
	ctx context.Context,
	operation ports.Operation[Value],
) ports.Result[Value] {
	return do(ctx, operation, &retrier.cfg)
}

func do[Value any](
	ctx context.Context,
	operation ports.Operation[Value],
	cfg *Config,
) ports.Result[Value] {
	ctx, cancel := retryContext(ctx, cfg)
	defer cancel()

	value, err := retry(ctx, operation, cfg)
	if err != nil {
		var zero Value

		return ports.Result[Value]{Value: zero, Err: translate(err)}
	}

	return ports.Result[Value]{Value: value, Err: nil}
}

// runOnce performs a single attempt, translating an error marked with
// [Permanent] into the retry engine's own stop signal.
func runOnce[Value any](ctx context.Context, operation ports.Operation[Value]) (Value, error) {
	outcome := operation(ctx)
	if outcome.Err == nil {
		return outcome.Value, nil
	}

	if permanent, ok := errors.AsType[domain.PermanentError](outcome.Err); ok {
		// Retry finds the marker with errors.As and replaces this error with a
		// RetryError carrying permanent.Err, so this message never reaches a
		// caller.
		return outcome.Value, fmt.Errorf(
			"backoffretry: stop retrying: %w",
			backoff.Permanent(permanent.PermanentCause()),
		)
	}

	return outcome.Value, outcome.Err
}

// notify builds the callback the retry engine invokes between attempts. It
// reads attempt through a pointer because the counter advances on every
// attempt, after this callback is built.
func notify(cfg *Config, attempt *uint) func(error, time.Duration) {
	return func(err error, delay time.Duration) {
		if cfg.Observer == nil {
			return
		}

		cfg.Observer(domain.RetryEvent{
			Attempt:   *attempt,
			Err:       err,
			NextDelay: delay,
		})
	}
}

func translate(err error) error {
	retryErr := backoff.AsRetryError(err)
	if retryErr == nil {
		return newInitError(domain.FailureExhausted, err, nil)
	}

	return newInitError(stopReason(retryErr.Cause), retryErr.LastErr, retryErr.Cause)
}

//nolint:ireturn // Context wrappers must retain the standard context interface.
func retryContext(ctx context.Context, cfg *Config) (context.Context, context.CancelFunc) {
	if cfg.Timeout > noDuration {
		return context.WithTimeout(ctx, cfg.Timeout)
	}

	return ctx, func() {}
}

func policy(cfg *Config) *backoff.ExponentialBackOff {
	policy := backoff.NewExponentialBackOff()

	policy.InitialInterval = cfg.InitialInterval
	policy.MaxInterval = cfg.MaxInterval
	policy.Multiplier = multiplier
	policy.RandomizationFactor = randomizationFactor

	return policy
}

func stopReason(cause error) domain.FailureReason {
	switch {
	case errors.Is(cause, backoff.ErrPermanent):
		return domain.FailurePermanent
	case errors.Is(cause, context.DeadlineExceeded), errors.Is(cause, backoff.ErrMaxElapsedTime):
		return domain.FailureTimedOut
	case errors.Is(cause, context.Canceled):
		return domain.FailureCanceled
	default:
		return domain.FailureExhausted
	}
}

func retry[Value any](
	ctx context.Context,
	operation ports.Operation[Value],
	cfg *Config,
) (Value, error) {
	var attempt uint

	value, err := backoff.Retry(
		ctx,
		func() (Value, error) {
			attempt++

			return runOnce(ctx, operation)
		},
		retryOptions(cfg, &attempt)...,
	)
	if err != nil {
		return value, fmt.Errorf("backoffretry: retry operation: %w", err)
	}

	return value, nil
}

func withBackOff(policy backOff) backoff.RetryOption { return backoff.WithBackOff(policy) }

func retryOptions(cfg *Config, attempt *uint) []backoff.RetryOption {
	return []backoff.RetryOption{
		withBackOff(policy(cfg)),
		backoff.WithMaxTries(cfg.MaxAttempts),
		backoff.WithMaxElapsedTime(noDuration),
		backoff.WithNotify(notify(cfg, attempt)),
	}
}

// newInitError builds a classified initialization failure.
//
// cause is the retry policy's own stop condition rather than anything inferred
// from err. It is not exported on the result but stays reachable through
// [initError.Unwrap], so no retry library leaks into the public API.
func newInitError(reason domain.FailureReason, err, cause error) *initError {
	return &initError{
		details: failureDetails{reason: reason, err: err, chain: errorChain(err, cause)},
	}
}

// Error formats the reason and the factory error as
// "singleton: <reason>: <err>".
func (failure *initError) Error() string {
	return fmt.Sprintf("singleton: %s: %v", reasonName(failure.details.reason), failure.details.err)
}

// Unwrap returns the factory error together with the retry policy's stop
// condition, so errors.Is and errors.As match either.
//
// Because Unwrap returns a slice, the single-error errors.Unwrap returns nil
// for an initError.
func (failure *initError) Unwrap() []error {
	return unwrapFailure(&failure.details)
}

// Err returns the final factory error.
func (failure *initError) Err() error { return failure.details.err }

// Reason returns the policy's stop condition.
func (failure *initError) Reason() domain.FailureReason { return failure.details.reason }

// Error returns the wrapped error's message.
func (failure *permanentError) Error() string {
	return failure.err.Error()
}

// Unwrap returns the wrapped error.
func (failure *permanentError) Unwrap() error {
	return failure.err
}

// Permanent marks a factory error as non-retriable.
//
// Permanent(nil) returns nil.
func Permanent(err error) error {
	if err == nil {
		return nil
	}

	return &permanentError{err: err}
}

// PermanentCause returns the non-retriable factory error.
func (failure *permanentError) PermanentCause() error { return failure.err }

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
