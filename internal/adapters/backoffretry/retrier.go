// Package backoffretry adapts github.com/cenkalti/backoff to the singleton
// Retrier port.
//
// It is the only package that imports a retry engine. Everything the engine
// reports is translated into domain types here, so no part of the core — and
// no caller — depends on its error model.
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

const (
	multiplier          = 2
	randomizationFactor = 0.2
)

// Config tunes the retry policy.
type Config struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts uint

	// Timeout bounds the whole retry loop. Zero disables it.
	Timeout time.Duration

	// InitialInterval is the first delay between attempts.
	InitialInterval time.Duration

	// MaxInterval is the ceiling the delay grows toward.
	MaxInterval time.Duration

	// Observer receives one event per retried attempt. It must already be
	// panic-safe; see [domain.RetryObserver.Safe].
	Observer domain.RetryObserver
}

// Retrier runs an operation with exponential backoff and jitter.
//
// It implements [ports.Retrier].
type Retrier[T any] struct {
	cfg Config
}

// New builds a Retrier for values of type T.
func New[T any](cfg Config) *Retrier[T] {
	return &Retrier[T]{cfg: cfg}
}

var _ ports.Retrier[int] = (*Retrier[int])(nil)

// Do runs op until it succeeds or the policy stops.
//
// On failure it returns the zero T and a *domain.InitError whose Reason comes
// from the policy's own stop condition rather than from the operation's error.
func (r *Retrier[T]) Do(ctx context.Context, operation ports.Operation[T]) (T, error) {
	cancel := func() {}

	if r.cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, r.cfg.Timeout)
	}

	defer cancel()

	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = r.cfg.InitialInterval
	policy.MaxInterval = r.cfg.MaxInterval
	policy.Multiplier = multiplier
	policy.RandomizationFactor = randomizationFactor

	var attempt uint

	value, err := backoff.Retry(
		ctx,
		func() (T, error) {
			attempt++

			return runOnce(ctx, operation)
		},
		backoff.WithBackOff(policy),
		backoff.WithMaxTries(r.cfg.MaxAttempts),
		backoff.WithMaxElapsedTime(0),
		backoff.WithNotify(r.notify(&attempt)),
	)
	if err != nil {
		var zero T

		return zero, translate(err)
	}

	return value, nil
}

// runOnce performs a single attempt, translating an error marked with
// [domain.Permanent] into the retry engine's own stop signal.
func runOnce[T any](ctx context.Context, operation ports.Operation[T]) (T, error) {
	value, err := operation(ctx)
	if err == nil {
		return value, nil
	}

	if permanent, ok := errors.AsType[*domain.PermanentError](err); ok {
		// Retry finds the marker with errors.As and replaces this error with a
		// RetryError carrying permanent.Err, so this message never reaches a
		// caller.
		return value, fmt.Errorf(
			"backoffretry: stop retrying: %w",
			backoff.Permanent(permanent.Err),
		)
	}

	return value, err
}

// notify builds the callback the retry engine invokes between attempts. It
// reads attempt through a pointer because the counter advances on every
// attempt, after this callback is built.
func (r *Retrier[T]) notify(attempt *uint) func(error, time.Duration) {
	return func(err error, delay time.Duration) {
		if r.cfg.Observer == nil {
			return
		}

		r.cfg.Observer(domain.RetryEvent{
			Attempt:   *attempt,
			Err:       err,
			NextDelay: delay,
		})
	}
}

func translate(err error) error {
	retryErr := backoff.AsRetryError(err)
	if retryErr == nil {
		return domain.NewInitError(domain.FailureExhausted, err, nil)
	}

	reason := domain.FailureExhausted

	switch {
	case errors.Is(retryErr.Cause, backoff.ErrPermanent):
		reason = domain.FailurePermanent

	case errors.Is(retryErr.Cause, context.DeadlineExceeded),
		errors.Is(retryErr.Cause, backoff.ErrMaxElapsedTime):
		reason = domain.FailureTimedOut

	case errors.Is(retryErr.Cause, context.Canceled):
		reason = domain.FailureCanceled
	}

	return domain.NewInitError(reason, retryErr.LastErr, retryErr.Cause)
}
