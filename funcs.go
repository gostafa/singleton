// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mostafakhairy0305-dot/singleton/internal/adapters/backoffretry"
	"github.com/mostafakhairy0305-dot/singleton/internal/application"
	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
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

// Error formats the reason and the factory error as "singleton: <reason>: <err>".
func (e *InitError) Error() string {
	return fmt.Sprintf("singleton: %s: %v", e.Reason, e.Err)
}

// Unwrap returns the factory error together with the retry policy's stop
// condition: ErrPermanent, ErrRetriesExhausted, context.DeadlineExceeded, or
// context.Canceled. A manually constructed InitError unwraps only Err.
//
// Because Unwrap returns a slice, errors.Unwrap returns nil for an InitError.
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

func defaultConfig() config {
	return config{
		retry: backoffretry.Config{
			MaxAttempts:     defaultMaxAttempts,
			Timeout:         defaultTimeout,
			InitialInterval: defaultInitialInterval,
			MaxInterval:     defaultMaxInterval,

			// No observer until [WithRetryObserver] registers one.
			Observer: nil,
		},
	}
}

// WithMaxAttempts sets the total number of attempts, including the first, so
// WithMaxAttempts(1) never retries. The default is 5.
//
// [New] reports [ErrZeroMaxAttempts] if attempts is zero.
func WithMaxAttempts(attempts uint) Option {
	return Option{
		apply: func(cfg *config) error {
			if attempts == 0 {
				return ErrZeroMaxAttempts
			}

			cfg.retry.MaxAttempts = attempts

			return nil
		},
	}
}

// WithInitializationTimeout limits the complete shared initialization, covering
// every attempt and the waits between them. The default is 30 seconds, and a
// zero duration disables the timeout.
//
// Exceeding it reports [FailureTimedOut]. [New] returns [ErrNegativeTimeout]
// if timeout is negative.
func WithInitializationTimeout(timeout time.Duration) Option {
	return Option{
		apply: func(cfg *config) error {
			if timeout < 0 {
				return ErrNegativeTimeout
			}

			cfg.retry.Timeout = timeout

			return nil
		},
	}
}

// WithRetryInterval sets the first delay between attempts and the ceiling that
// delay grows toward. Delays grow exponentially from initial, are capped at
// maximum, and carry jitter. The defaults are 250ms and 5s.
//
// [New] reports [ErrZeroInitialInterval] if initial is not positive, or
// [ErrMaxIntervalBelowInitial] if maximum is less than initial.
func WithRetryInterval(initial, maximum time.Duration) Option {
	return Option{
		apply: func(cfg *config) error {
			if initial <= 0 {
				return ErrZeroInitialInterval
			}

			if maximum < initial {
				return ErrMaxIntervalBelowInitial
			}

			cfg.retry.InitialInterval = initial
			cfg.retry.MaxInterval = maximum

			return nil
		},
	}
}

// WithRetryObserver registers a function called once per retried attempt.
//
// A run that succeeds on its third attempt delivers two events; the final
// attempt delivers none, because its outcome is the result of Get.
//
// The observer should return quickly. A panic in it is recovered and discarded,
// so instrumentation cannot become the singleton's result. Passing nil disables
// it.
func WithRetryObserver(observer func(RetryEvent)) Option {
	return Option{
		apply: func(cfg *config) error {
			cfg.retry.Observer = internalObserver(observer)

			return nil
		},
	}
}

func internalObserver(observer func(RetryEvent)) domain.RetryObserver {
	if observer == nil {
		return nil
	}

	return func(event domain.RetryEvent) {
		observer(RetryEvent{
			Attempt:   event.Attempt,
			Err:       event.Err,
			NextDelay: event.NextDelay,
		})
	}
}

func (r publicRetrier[T]) Do(ctx context.Context, operation ports.Operation[T]) (T, error) {
	value, err := r.retrier.Do(ctx, internalOperation(operation))
	if err == nil {
		return value, nil
	}

	return value, publicInitError(err)
}

func internalOperation[T any](operation ports.Operation[T]) ports.Operation[T] {
	return func(ctx context.Context) (T, error) {
		value, err := operation(ctx)
		if permanent, ok := errors.AsType[*PermanentError](err); ok {
			return value, &domain.PermanentError{Err: permanent.Err}
		}

		return value, err
	}
}

func publicInitError(err error) *InitError {
	internal, ok := errors.AsType[*domain.InitError](err)
	if !ok {
		return &InitError{
			Reason: FailureExhausted,
			Err:    err,
			chain:  []error{err, ErrRetriesExhausted},
		}
	}

	reason, cause := publicFailure(internal.Reason)

	initErr := &InitError{Reason: reason, Err: internal.Err, chain: nil}
	if internal.Err != nil {
		initErr.chain = append(initErr.chain, internal.Err)
	}

	if cause != nil {
		initErr.chain = append(initErr.chain, cause)
	}

	return initErr
}

func publicFailure(reason domain.FailureReason) (FailureReason, error) {
	switch reason {
	case domain.FailurePermanent:
		return FailurePermanent, ErrPermanent
	case domain.FailureExhausted:
		return FailureExhausted, ErrRetriesExhausted
	case domain.FailureTimedOut:
		return FailureTimedOut, context.DeadlineExceeded
	case domain.FailureCanceled:
		return FailureCanceled, context.Canceled
	}

	return 0, nil
}

// Get waits for and returns the shared value, starting initialization on the
// first call and blocking until it settles. It returns the zero T and an
// [InitError] if initialization failed, or the zero T and an error wrapping
// context.Cause(ctx) if the caller's context ended first, which errors.Is
// matches against the cause. It re-panics with the factory's panic value
// if the factory panicked, and panics if ctx is nil or if the Provider is the
// zero value. A failed initialization is cached and returned to every later
// caller until Reset is called.
func (p *Provider[T]) Get(ctx context.Context) (T, error) {
	if ctx == nil {
		panic("singleton: nil context")
	}

	if p.provider == nil {
		panic("singleton: Provider must be created with New or MustNew")
	}

	return p.provider.Get(ctx) //nolint:wrapcheck // Preserve the cached public error's identity.
}

// Reset discards a failed or panicked initialization so the next Get starts a
// new one. It does nothing after a success, nothing while initialization is in
// flight, and nothing before the first Get.
func (p *Provider[T]) Reset() {
	if p.provider != nil {
		p.provider.Reset()
	}
}

// Permanent marks a factory error as non-retriable. Initialization stops at
// that attempt and reports [FailurePermanent].
//
// Permanent(nil) returns nil. The wrapped error stays reachable through
// errors.Is and errors.As. The concrete type is [PermanentError].
func Permanent(err error) error {
	if err == nil {
		return nil
	}

	return &PermanentError{Err: err}
}

// New creates a lazy singleton provider.
//
// It does not call factory; the first Get does.
//
// New returns [ErrNilFactory] if factory is nil, [ErrInvalidOption] if a
// zero-value [Option] is passed, or the option's validation error if an option
// is invalid. It never reports factory failures, which surface from Get instead.
func New[T any](
	factory Factory[T],
	options ...Option,
) (*Provider[T], error) {
	if factory == nil {
		return nil, ErrNilFactory
	}

	cfg := defaultConfig()

	for _, option := range options {
		if option.apply == nil {
			return nil, ErrInvalidOption
		}

		err := option.apply(&cfg)
		if err != nil {
			return nil, err
		}
	}

	cfg.retry.Observer = cfg.retry.Observer.Safe()

	return &Provider[T]{
		provider: application.NewProvider(
			ports.Operation[T](factory),
			publicRetrier[T]{retrier: backoffretry.New[T](cfg.retry)},
		),
	}, nil
}

// MustNew is like [New] but panics instead of returning an error, which suits
// package-level declarations.
//
// It panics only for invalid construction options, never for factory failures.
func MustNew[T any](
	factory Factory[T],
	options ...Option,
) *Provider[T] {
	provider, err := New(factory, options...)
	if err != nil {
		panic(err)
	}

	return provider
}
