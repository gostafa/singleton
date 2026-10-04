// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v7"
	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

// New builds a Retrier for values of type Value.
//
//nolint:gocritic // Copy configuration so later caller mutations cannot change the policy.
func New[Value any](cfg Config) *Retrier[Value] {
	return &Retrier[Value]{cfg: cfg}
}

// Do runs op until it succeeds or the policy stops.
//
// On failure it returns the zero Value and a [domain.InitError] whose Reason comes
// from the policy's own stop condition rather than from the operation's error.
func (retrier *Retrier[Value]) Do(
	ctx context.Context,
	operation ports.Operation[Value],
) (Value, error) {
	//nolint:wrapcheck // Preserve the classified initialization error.
	return do(ctx, operation, &retrier.cfg)
}

func do[Value any](
	ctx context.Context,
	operation ports.Operation[Value],
	cfg *Config,
) (Value, error) {
	ctx, cancel := retryContext(ctx, cfg)
	defer cancel()

	value, err := retry(ctx, operation, cfg)
	if err != nil {
		var zero Value

		//nolint:wrapcheck // Return the classified domain error directly.
		return zero, translate(err)
	}

	return value, nil
}

// runOnce performs a single attempt, translating an error marked with
// [Permanent] into the retry engine's own stop signal.
func runOnce[Value any](ctx context.Context, operation ports.Operation[Value]) (Value, error) {
	value, err := operation(ctx)
	if err == nil {
		return value, nil
	}

	if permanent, ok := errors.AsType[domain.PermanentError](err); ok {
		// Retry finds the marker with errors.As and replaces this error with a
		// RetryError carrying permanent.Err, so this message never reaches a
		// caller.
		return value, fmt.Errorf(
			"backoffretry: stop retrying: %w",
			backoff.Permanent(permanent.PermanentCause()),
		)
	}

	//nolint:wrapcheck // Keep the original factory error in retry events and results.
	return value, err
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
		return NewInitError(domain.FailureExhausted, err, nil)
	}

	return NewInitError(stopReason(retryErr.Cause), retryErr.LastErr, retryErr.Cause)
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

	//nolint:wrapcheck // Translate the retry engine error at the Do boundary.
	return backoff.Retry(
		ctx,
		func() (Value, error) {
			attempt++

			return runOnce(ctx, operation)
		},
		withBackOff(policy(cfg)),
		backoff.WithMaxTries(cfg.MaxAttempts),
		backoff.WithMaxElapsedTime(noDuration),
		backoff.WithNotify(notify(cfg, &attempt)),
	)
}

func withBackOff(policy backOff) backoff.RetryOption { return backoff.WithBackOff(policy) }
