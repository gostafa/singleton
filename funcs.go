// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gostafa/singleton/internal/singleton/adapters/backoffretry"
	"github.com/gostafa/singleton/internal/singleton/application"
	"github.com/gostafa/singleton/internal/singleton/domain"
	"github.com/gostafa/singleton/internal/singleton/ports"
)

// String returns a short human-readable description of reason.
func (reason FailureReason) String() string {
	names := [...]string{
		FailurePermanent: "permanent failure",
		FailureExhausted: "retries exhausted",
		FailureTimedOut:  "initialization timed out",
		FailureCanceled:  "initialization canceled",
	}

	if int(reason) >= len(names) || names[reason] == "" {
		return "initialization failed"
	}

	return names[reason]
}

// Error formats the reason and the factory error as "singleton: <reason>: <err>".
func (failure *InitError) Error() string {
	return fmt.Sprintf("singleton: %s: %v", failure.details.reason, failure.details.err)
}

// NewInitError constructs a failure that unwraps only err.
// Provider.Get adds its retry policy's stop condition to the returned failure.
func NewInitError(reason FailureReason, err error) *InitError {
	return &InitError{details: failureDetails{reason: reason, err: err, chain: nil}}
}

// Unwrap returns the factory error together with the retry policy's stop
// condition: ErrPermanent, ErrRetriesExhausted, context.DeadlineExceeded, or
// context.Canceled. A failure constructed with [NewInitError] unwraps only Err().
//
// Because Unwrap returns a slice, errors.Unwrap returns nil for an InitError.
func (failure *InitError) Unwrap() []error { return unwrapFailure(&failure.details) }

// Err returns the final factory error.
func (failure *InitError) Err() error { return failure.details.err }

// Reason returns the condition that stopped initialization.
func (failure *InitError) Reason() FailureReason { return failure.details.reason }

func unwrapFailure(details *failureDetails) []error {
	if details.chain != nil {
		return details.chain
	}

	return publicErrorChain(details.err, nil)
}

// Error returns the wrapped error's message.
func (failure *PermanentError) Error() string {
	return failure.Err.Error()
}

// Unwrap returns the wrapped error.
func (failure *PermanentError) Unwrap() error {
	return failure.Err
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
		apply: func(cfg *config) validationCode {
			if attempts == zeroAttempts {
				return zeroMaxAttempts
			}

			cfg.retry.MaxAttempts = attempts

			return validOptions
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
		apply: func(cfg *config) validationCode {
			if timeout < zeroAttempts {
				return negativeTimeout
			}

			cfg.retry.Timeout = timeout

			return validOptions
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
		apply: func(cfg *config) validationCode {
			if initial <= zeroAttempts {
				return zeroInitialInterval
			}

			if maximum < initial {
				return maxIntervalBelowInitial
			}

			cfg.retry.InitialInterval = initial
			cfg.retry.MaxInterval = maximum

			return validOptions
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
		apply: func(cfg *config) validationCode {
			cfg.retry.Observer = internalObserver(observer)

			return validOptions
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

// Do converts factory markers and preserves the shared public failure.
func (reason publicRetrier[Value]) Do(
	ctx context.Context,
	operation ports.Operation[Value],
) ports.Result[Value] {
	return publicResult(reason.retrier.Do(ctx, operation))
}

func internalOperation[Value any](operation Factory[Value]) ports.Operation[Value] {
	return func(ctx context.Context) ports.Result[Value] {
		value, err := operation(ctx)
		if permanent, ok := errors.AsType[*PermanentError](err); ok {
			return ports.Result[Value]{Value: value, Err: backoffretry.Permanent(permanent.Err)}
		}

		return ports.Result[Value]{Value: value, Err: err}
	}
}

func publicInitError(err error) *InitError {
	internal, ok := errors.AsType[domain.InitError](err)
	if !ok {
		return &InitError{
			details: failureDetails{
				reason: FailureExhausted,
				err:    err,
				chain:  []error{err, ErrRetriesExhausted},
			},
		}
	}

	reason, cause := publicFailure(internal.Reason())

	return &InitError{
		details: failureDetails{
			reason: reason,
			err:    internal.Err(),
			chain:  publicErrorChain(internal.Err(), cause),
		},
	}
}

func publicFailure(reason domain.FailureReason) (FailureReason, error) {
	failures := map[domain.FailureReason]struct {
		cause  error
		reason FailureReason
	}{
		domain.FailurePermanent: {ErrPermanent, FailurePermanent},
		domain.FailureExhausted: {ErrRetriesExhausted, FailureExhausted},
		domain.FailureTimedOut:  {context.DeadlineExceeded, FailureTimedOut},
		domain.FailureCanceled:  {context.Canceled, FailureCanceled},
	}
	failure := failures[reason]

	return failure.reason, failure.cause
}

// Get waits for and returns the shared value, starting initialization on the
// first call and blocking until it settles. It returns the zero Value and an
// [InitError] if initialization failed, or the zero Value and an error wrapping
// context.Cause(ctx) if the caller's context ended first, which errors.Is
// matches against the cause. It re-panics with the factory's panic value
// if the factory panicked, and panics if ctx is nil or if the Provider is the
// zero value. A failed initialization is cached and returned to every later
// caller until Reset is called.
func (provider *Provider[Value]) Get(ctx context.Context) (Value, error) {
	outcome := get(ctx, provider.provider)

	return outcome.Value, outcome.Err
}

func get[Value any](
	ctx context.Context,
	provider application.Interface[Value],
) ports.Result[Value] {
	if ctx == nil {
		//nolint:forbidigo // Preserve the documented panic contract.
		panic("singleton: nil context")
	}

	if provider == nil {
		//nolint:forbidigo // Preserve the documented panic contract.
		panic("singleton: Provider must be created with New or MustNew")
	}

	return provider.Get(ctx)
}

// Reset discards a failed or panicked initialization so the next Get starts a
// new one. It does nothing after a success, nothing while initialization is in
// flight, and nothing before the first Get.
func (provider *Provider[Value]) Reset() {
	reset(provider.provider)
}

func reset[Value any](provider application.Interface[Value]) {
	if provider != nil {
		provider.Reset()
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
func New[Value any](
	factory Factory[Value],
	options ...Option,
) (*Provider[Value], error) {
	if factory == nil {
		return nil, ErrNilFactory
	}

	cfg := defaultConfig()

	code := applyOptions(&cfg, options)
	if code != validOptions {
		return nil, validationErrors()[code]
	}

	cfg.retry.Observer = domain.Safe(cfg.retry.Observer)

	return configuredProvider(factory, &cfg), nil
}

// MustNew is like [New] but panics instead of returning an error, which suits
// package-level declarations.
//
// It panics only for invalid construction options, never for factory failures.
func MustNew[Value any](
	factory Factory[Value],
	options ...Option,
) *Provider[Value] {
	provider, err := New(factory, options...)
	if err != nil {
		//nolint:forbidigo // Preserve the documented panic contract.
		panic(err)
	}

	return provider
}

func applyOptions(cfg *config, options []Option) validationCode {
	for index := range options {
		if options[index].apply == nil {
			return invalidOption
		}

		code := options[index].applyTo(cfg)
		if code != validOptions {
			return code
		}
	}

	return validOptions
}

func publicErrorChain(err, cause error) []error {
	var chain []error

	if err != nil {
		chain = append(chain, err)
	}

	if cause != nil {
		chain = append(chain, cause)
	}

	return chain
}

func configuredProvider[Value any](factory Factory[Value], cfg *config) *Provider[Value] {
	return &Provider[Value]{
		provider: application.NewProvider(
			internalOperation(factory),
			publicRetrier[Value]{retrier: backoffretry.New[Value](cfg.retry)},
		),
	}
}

func validationErrors() map[validationCode]error {
	return map[validationCode]error{
		validOptions:            nil,
		invalidOption:           ErrInvalidOption,
		zeroMaxAttempts:         ErrZeroMaxAttempts,
		negativeTimeout:         ErrNegativeTimeout,
		zeroInitialInterval:     ErrZeroInitialInterval,
		maxIntervalBelowInitial: ErrMaxIntervalBelowInitial,
	}
}

func (option Option) applyTo(cfg *config) validationCode {
	return option.apply(cfg)
}

func publicResult[Value any](outcome ports.Result[Value]) ports.Result[Value] {
	if outcome.Err != nil {
		outcome.Err = publicInitError(outcome.Err)
	}

	return outcome
}
