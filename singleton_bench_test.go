// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mostafakhairy0305-dot/singleton"
)

var errBench = errors.New("bench failure")

type benchConstructionCase struct {
	name    string
	factory singleton.Factory[int]
	options []singleton.Option
	want    error
}

type benchInitErrorCase struct {
	name    string
	failure *singleton.InitError
	cause   error
	stop    error
	reason  singleton.FailureReason
}

type benchGetResult struct {
	value int
	err   error
}

func benchConstructionCases() []benchConstructionCase {
	return []benchConstructionCase{
		{name: "default", factory: successfulFactory},
		{name: "with_options", factory: successfulFactory, options: append(benchOptions(),
			singleton.WithRetryObserver(func(singleton.RetryEvent) {}))},
		{
			name: "disabled_timeout", factory: successfulFactory,
			options: []singleton.Option{singleton.WithInitializationTimeout(0)},
		},
		{
			name: "nil_observer", factory: successfulFactory,
			options: []singleton.Option{singleton.WithRetryObserver(nil)},
		},
		{
			name: "single_attempt", factory: successfulFactory,
			options: []singleton.Option{singleton.WithMaxAttempts(1)},
		},
		{
			name:    "growing_retry_interval",
			factory: successfulFactory,
			options: []singleton.Option{
				singleton.WithRetryInterval(time.Microsecond, time.Millisecond),
			},
		},
		{name: "nil_factory", want: singleton.ErrNilFactory},
		{
			name: "zero_option", factory: successfulFactory,
			options: []singleton.Option{{}}, want: singleton.ErrInvalidOption,
		},
		{
			name:    "zero_attempts",
			factory: successfulFactory,
			options: []singleton.Option{
				singleton.WithMaxAttempts(0),
			},
			want: singleton.ErrZeroMaxAttempts,
		},
		{
			name:    "negative_timeout",
			factory: successfulFactory,
			options: []singleton.Option{
				singleton.WithInitializationTimeout(-time.Second),
			},
			want: singleton.ErrNegativeTimeout,
		},
		{
			name:    "zero_interval",
			factory: successfulFactory,
			options: []singleton.Option{
				singleton.WithRetryInterval(0, time.Second),
			},
			want: singleton.ErrZeroInitialInterval,
		},
		{
			name:    "negative_interval",
			factory: successfulFactory,
			options: []singleton.Option{
				singleton.WithRetryInterval(-time.Second, time.Second),
			},
			want: singleton.ErrZeroInitialInterval,
		},
		{
			name:    "maximum_below_initial",
			factory: successfulFactory,
			options: []singleton.Option{
				singleton.WithRetryInterval(time.Second, time.Millisecond),
			},
			want: singleton.ErrMaxIntervalBelowInitial,
		},
	}
}

func checkBenchConstruction(b *testing.B, provider *singleton.Provider[int], err, want error) {
	b.Helper()
	if want != nil {
		if provider != nil || err != want {
			b.Fatalf("New() = (%v, %v), want (nil, %v)", provider, err, want)
		}
		return
	}
	if provider == nil || err != nil {
		b.Fatalf("construction = (%v, %v), want a provider", provider, err)
	}
	value, err := provider.Get(b.Context())
	if value != 42 || err != nil {
		b.Fatalf("Get() = (%d, %v), want (42, nil)", value, err)
	}
}

// benchPanic verifies both the presence and identity of a documented panic.
func benchPanic(b *testing.B, call func(), want any) {
	b.Helper()
	defer func() {
		if got := recover(); got != want {
			b.Fatalf("panic = %v, want %v", got, want)
		}
	}()
	call()
	b.Fatal("call did not panic")
}

func benchFailure(
	b *testing.B,
	value int,
	err error,
	reason singleton.FailureReason,
	cause error,
) *singleton.InitError {
	b.Helper()
	failure, ok := errors.AsType[*singleton.InitError](err)
	if !ok || value != 0 {
		b.Fatalf("Get() = (%d, %v), want (0, *InitError)", value, err)
	}
	if failure.Reason() != reason || failure.Err() != errBench || !errors.Is(err, cause) ||
		!errors.Is(err, errBench) {
		b.Fatalf("unexpected failure: %v", failure)
	}
	return failure
}

func benchOptions() []singleton.Option {
	return []singleton.Option{
		singleton.WithMaxAttempts(3),
		singleton.WithInitializationTimeout(time.Second),
		singleton.WithRetryInterval(time.Microsecond, time.Microsecond),
	}
}

// BenchmarkGetInitialized measures the hot path: Get on a provider whose
// value is already cached.
func BenchmarkGetInitialized(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	provider := singleton.MustNew(successfulFactory)

	value, err := provider.Get(ctx)
	if value != 42 || err != nil {
		b.Fatalf("Get() = (%d, %v), want (42, nil)", value, err)
	}

	for b.Loop() {
		value, err = provider.Get(ctx)
		if value != 42 || err != nil {
			b.Fatalf("Get() = (%d, %v), want (42, nil)", value, err)
		}
	}
}

// BenchmarkGetInitializedParallel measures the cached hot path under
// concurrent callers.
func BenchmarkGetInitializedParallel(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	provider := singleton.MustNew(successfulFactory)

	_, err := provider.Get(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			value, getErr := provider.Get(ctx)
			if getErr != nil || value != 42 {
				b.Errorf("Get() = (%d, %v), want (42, nil)", value, getErr)

				return
			}
		}
	})
}

// BenchmarkNew measures provider construction with and without options.
func BenchmarkNew(b *testing.B) {
	for _, scenario := range benchConstructionCases() {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			factory, calls := benchLazyFactory(scenario.factory)
			var provider *singleton.Provider[int]
			var err error
			for b.Loop() {
				provider, err = singleton.New(factory, scenario.options...)
			}
			if calls.Load() != 0 {
				b.Fatal("New called the factory during construction")
			}
			checkBenchConstruction(b, provider, err, scenario.want)
		})
	}
}

// BenchmarkFirstGet measures a cold start: construct a provider and run its
// first successful initialization.
func BenchmarkFirstGet(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()

	for b.Loop() {
		provider := singleton.MustNew(successfulFactory)

		value, err := provider.Get(ctx)
		if value != 42 || err != nil {
			b.Fatalf("Get() = (%d, %v), want (42, nil)", value, err)
		}
	}
}

// BenchmarkFirstGetConcurrentCallers measures a cold start where several
// goroutines race to read the same uninitialized provider.
func BenchmarkFirstGetConcurrentCallers(b *testing.B) {
	b.ReportAllocs()
	const callers = 8

	ctx := context.Background()
	results := make(chan benchGetResult, callers)

	for b.Loop() {
		provider := singleton.MustNew(successfulFactory)

		for range callers {
			go func() {
				value, err := provider.Get(ctx)
				results <- benchGetResult{value: value, err: err}
			}()
		}

		var unexpected bool
		for range callers {
			result := <-results
			unexpected = unexpected || result.value != 42 || result.err != nil
		}
		if unexpected {
			b.Fatal("concurrent Get returned an unexpected value or error")
		}
	}
}

// BenchmarkPermanentFailure measures an initialization that stops on its
// first attempt because the factory returned a permanent error.
func BenchmarkPermanentFailure(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	factory := func(context.Context) (int, error) { return 0, singleton.Permanent(errBench) }

	for b.Loop() {
		provider := singleton.MustNew(factory, benchOptions()...)

		_, err := provider.Get(ctx)
		if err == nil {
			b.Fatal("expected an error")
		}
	}
}

// BenchmarkRetryUntilSuccess measures an initialization that fails twice and
// succeeds on its third attempt, exercising the retry loop and observer.
func BenchmarkRetryUntilSuccess(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	options := append(benchOptions(), singleton.WithRetryObserver(func(singleton.RetryEvent) {}))

	for b.Loop() {
		attempts := 0
		provider := singleton.MustNew(func(context.Context) (int, error) {
			attempts++
			if attempts < 3 {
				return 0, errBench
			}

			return attempts, nil
		}, options...)

		value, err := provider.Get(ctx)
		if err != nil || value != 3 || attempts != 3 {
			b.Fatalf("Get() = (%d, %v), attempts = %d", value, err, attempts)
		}
	}
}

// BenchmarkGetCachedFailure measures Get on a provider whose failed
// initialization is cached.
func BenchmarkGetCachedFailure(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, singleton.Permanent(errBench)
	}, benchOptions()...)

	_, err := provider.Get(ctx)
	if err == nil {
		b.Fatal("expected an error")
	}

	cached := err
	for b.Loop() {
		_, err = provider.Get(ctx)
		if err != cached {
			b.Fatal("cached error identity changed")
		}
	}
}

// BenchmarkResetAndReinitialize measures discarding a failed initialization
// with Reset and running a fresh one.
func BenchmarkResetAndReinitialize(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, singleton.Permanent(errBench)
	}, benchOptions()...)
	value, err := provider.Get(ctx)
	benchFailure(b, value, err, singleton.FailurePermanent, singleton.ErrPermanent)

	for b.Loop() {
		provider.Reset()

		value, err := provider.Get(ctx)
		benchFailure(b, value, err, singleton.FailurePermanent, singleton.ErrPermanent)
	}
}

// BenchmarkInitErrorInspection measures classifying a returned failure with
// errors.Is, errors.As and Error().
func BenchmarkInitErrorInspection(b *testing.B) {
	b.ReportAllocs()
	ctx := context.Background()
	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, singleton.Permanent(errBench)
	}, benchOptions()...)

	_, err := provider.Get(ctx)
	if err == nil {
		b.Fatal("expected an error")
	}

	for b.Loop() {
		if !errors.Is(err, singleton.ErrPermanent) || !errors.Is(err, errBench) {
			b.Fatal("unexpected error chain")
		}

		failure, ok := errors.AsType[*singleton.InitError](err)
		if !ok || failure.Reason() != singleton.FailurePermanent {
			b.Fatal("unexpected failure")
		}

		if failure.Error() == "" {
			b.Fatal("empty message")
		}
	}
}

// BenchmarkMustNew measures construction and recovery of validation panics.
func BenchmarkMustNew(b *testing.B) {
	for _, scenario := range benchConstructionCases() {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			factory, calls := benchLazyFactory(scenario.factory)
			var provider *singleton.Provider[int]
			for b.Loop() {
				if scenario.want == nil {
					provider = singleton.MustNew(factory, scenario.options...)
				} else {
					benchPanic(b, func() {
						provider = singleton.MustNew(factory, scenario.options...)
					}, scenario.want)
				}
			}
			if calls.Load() != 0 {
				b.Fatal("MustNew called the factory during construction")
			}
			if scenario.want == nil {
				checkBenchConstruction(b, provider, nil, nil)
			}
		})
	}
}

func benchLazyFactory(factory singleton.Factory[int]) (singleton.Factory[int], *atomic.Uint32) {
	calls := new(atomic.Uint32)
	if factory == nil {
		return nil, calls
	}
	return func(ctx context.Context) (int, error) {
		calls.Add(1)
		return factory(ctx)
	}, calls
}

// BenchmarkOptions measures creation alone; New benchmarks measure application.
func BenchmarkOptions(b *testing.B) {
	scenarios := []struct {
		name string
		make func() singleton.Option
		want error
	}{
		{
			"WithMaxAttempts/one",
			func() singleton.Option { return singleton.WithMaxAttempts(1) },
			nil,
		},
		{
			"WithMaxAttempts/multiple",
			func() singleton.Option { return singleton.WithMaxAttempts(3) },
			nil,
		},
		{
			"WithMaxAttempts/zero",
			func() singleton.Option { return singleton.WithMaxAttempts(0) },
			singleton.ErrZeroMaxAttempts,
		},
		{
			"WithInitializationTimeout/positive",
			func() singleton.Option { return singleton.WithInitializationTimeout(time.Second) },
			nil,
		},
		{
			"WithInitializationTimeout/disabled",
			func() singleton.Option { return singleton.WithInitializationTimeout(0) },
			nil,
		},
		{
			"WithInitializationTimeout/negative",
			func() singleton.Option { return singleton.WithInitializationTimeout(-time.Second) },
			singleton.ErrNegativeTimeout,
		},
		{
			"WithRetryInterval/capped",
			func() singleton.Option { return singleton.WithRetryInterval(time.Microsecond, time.Millisecond) },
			nil,
		},
		{
			"WithRetryInterval/equal",
			func() singleton.Option { return singleton.WithRetryInterval(time.Microsecond, time.Microsecond) },
			nil,
		},
		{
			"WithRetryInterval/zero",
			func() singleton.Option { return singleton.WithRetryInterval(0, time.Second) },
			singleton.ErrZeroInitialInterval,
		},
		{
			"WithRetryInterval/negative",
			func() singleton.Option { return singleton.WithRetryInterval(-time.Second, time.Second) },
			singleton.ErrZeroInitialInterval,
		},
		{
			"WithRetryInterval/invalid_maximum",
			func() singleton.Option { return singleton.WithRetryInterval(time.Second, time.Millisecond) },
			singleton.ErrMaxIntervalBelowInitial,
		},
		{
			"WithRetryObserver/enabled",
			func() singleton.Option { return singleton.WithRetryObserver(func(singleton.RetryEvent) {}) },
			nil,
		},
		{
			"WithRetryObserver/nil",
			func() singleton.Option { return singleton.WithRetryObserver(nil) },
			nil,
		},
	}
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			var option singleton.Option
			for b.Loop() {
				option = scenario.make()
			}
			provider, err := singleton.New(successfulFactory, option)
			checkBenchConstruction(b, provider, err, scenario.want)
		})
	}
}

func BenchmarkPermanent(b *testing.B) {
	for _, scenario := range []struct {
		name string
		err  error
	}{{"nil", nil}, {"error", errBench}} {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			var result error
			for b.Loop() {
				result = singleton.Permanent(scenario.err)
			}
			if scenario.err == nil {
				if result != nil {
					b.Fatal("Permanent(nil) must return nil")
				}
				return
			}
			marker, ok := errors.AsType[*singleton.PermanentError](result)
			if !ok || marker.Err != errBench || !errors.Is(result, errBench) {
				b.Fatalf("unexpected permanent marker: %v", result)
			}
		})
	}
}

func BenchmarkPermanentError(b *testing.B) {
	marker := &singleton.PermanentError{Err: errBench}
	b.Run("Error", func(b *testing.B) {
		b.ReportAllocs()
		var message string
		for b.Loop() {
			message = marker.Error()
		}
		if message != errBench.Error() {
			b.Fatal("unexpected permanent error message")
		}
	})
	b.Run("Unwrap", func(b *testing.B) {
		b.ReportAllocs()
		var cause error
		for b.Loop() {
			cause = marker.Unwrap()
		}
		if cause != errBench {
			b.Fatal("unexpected permanent error cause")
		}
	})
}

func BenchmarkFailureReasonString(b *testing.B) {
	scenarios := []struct {
		name   string
		reason singleton.FailureReason
		want   string
	}{
		{"permanent", singleton.FailurePermanent, "permanent failure"},
		{"exhausted", singleton.FailureExhausted, "retries exhausted"},
		{"timed_out", singleton.FailureTimedOut, "initialization timed out"},
		{"canceled", singleton.FailureCanceled, "initialization canceled"},
		{"zero", 0, "initialization failed"},
		{"unknown", singleton.FailureCanceled + 1, "initialization failed"},
		{"maximum", 255, "initialization failed"},
	}
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.ReportAllocs()
			var message string
			for b.Loop() {
				message = scenario.reason.String()
			}
			if message != scenario.want {
				b.Fatalf("String() = %q, want %q", message, scenario.want)
			}
		})
	}
}

func BenchmarkNewInitError(b *testing.B) {
	for _, reason := range []singleton.FailureReason{
		singleton.FailurePermanent, singleton.FailureExhausted,
		singleton.FailureTimedOut, singleton.FailureCanceled, 0, 255,
	} {
		for _, cause := range []error{nil, errBench} {
			b.Run(fmt.Sprintf("reason_%d/nil_cause_%t", reason, cause == nil), func(b *testing.B) {
				b.ReportAllocs()
				var failure *singleton.InitError
				for b.Loop() {
					failure = singleton.NewInitError(reason, cause)
				}
				if failure.Reason() != reason || failure.Err() != cause ||
					errors.Unwrap(failure) != nil {
					b.Fatal("unexpected constructed failure")
				}
				checkBenchChain(b, failure.Unwrap(), cause)
			})
		}
	}
}

func checkBenchChain(b *testing.B, chain []error, causes ...error) {
	b.Helper()
	var index int
	for _, cause := range causes {
		if cause == nil {
			continue
		}
		if index >= len(chain) || chain[index] != cause {
			b.Fatalf("Unwrap() = %v, want %v", chain, causes)
		}
		index++
	}
	if index != len(chain) {
		b.Fatalf("Unwrap() has %d causes, want %d", len(chain), index)
	}
}

func BenchmarkInitError(b *testing.B) {
	scenarios := []benchInitErrorCase{
		{name: "zero", failure: new(singleton.InitError)},
	}
	for _, reason := range []singleton.FailureReason{
		singleton.FailurePermanent, singleton.FailureExhausted,
		singleton.FailureTimedOut, singleton.FailureCanceled, 255,
	} {
		for _, cause := range []error{nil, errBench} {
			scenarios = append(scenarios, benchInitErrorCase{
				fmt.Sprintf("constructed_%d/nil_cause_%t", reason, cause == nil),
				singleton.NewInitError(reason, cause), cause, nil, reason,
			})
		}
	}
	for _, scenario := range []struct {
		name    string
		factory singleton.Factory[int]
		options []singleton.Option
		reason  singleton.FailureReason
		stop    error
	}{
		{
			"provider_permanent", func(context.Context) (int, error) { return 99, singleton.Permanent(errBench) },
			benchOptions(), singleton.FailurePermanent, singleton.ErrPermanent,
		},
		{
			"provider_exhausted", func(context.Context) (int, error) { return 99, errBench },
			[]singleton.Option{singleton.WithMaxAttempts(1)},
			singleton.FailureExhausted, singleton.ErrRetriesExhausted,
		},
		{
			"provider_timed_out", func(ctx context.Context) (int, error) {
				<-ctx.Done()
				return 99, errBench
			},
			[]singleton.Option{singleton.WithInitializationTimeout(time.Millisecond)},
			singleton.FailureTimedOut, context.DeadlineExceeded,
		},
	} {
		provider := singleton.MustNew(scenario.factory, scenario.options...)
		value, err := provider.Get(b.Context())
		failure := benchFailure(b, value, err, scenario.reason, scenario.stop)
		scenarios = append(
			scenarios,
			benchInitErrorCase{scenario.name, failure, errBench, scenario.stop, scenario.reason},
		)
	}
	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			b.Run("Error", func(b *testing.B) {
				b.ReportAllocs()
				want := fmt.Sprintf("singleton: %s: %v", scenario.reason, scenario.cause)
				var message string
				for b.Loop() {
					message = scenario.failure.Error()
				}
				if message != want {
					b.Fatalf("Error() = %q, want %q", message, want)
				}
			})
			b.Run("Unwrap", func(b *testing.B) {
				b.ReportAllocs()
				var chain []error
				for b.Loop() {
					chain = scenario.failure.Unwrap()
				}
				checkBenchChain(b, chain, scenario.cause, scenario.stop)
			})
			b.Run("Err", func(b *testing.B) {
				b.ReportAllocs()
				var cause error
				for b.Loop() {
					cause = scenario.failure.Err()
				}
				if cause != scenario.cause {
					b.Fatalf("Err() = %v, want %v", cause, scenario.cause)
				}
			})
			b.Run("Reason", func(b *testing.B) {
				b.ReportAllocs()
				var reason singleton.FailureReason
				for b.Loop() {
					reason = scenario.failure.Reason()
				}
				if reason != scenario.reason {
					b.Fatalf("Reason() = %v, want %v", reason, scenario.reason)
				}
			})
		})
	}
}

// BenchmarkGetFailure measures construction plus failed initialization.
func BenchmarkGetFailure(b *testing.B) {
	for _, kind := range []string{"permanent_helper", "permanent_literal", "permanent_wrapped", "exhausted", "timed_out"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			options := benchOptions()
			reason, stop := singleton.FailurePermanent, singleton.ErrPermanent
			marker := singleton.Permanent(errBench)
			if kind == "permanent_literal" {
				marker = &singleton.PermanentError{Err: errBench}
			}
			if kind == "permanent_wrapped" {
				marker = fmt.Errorf("factory: %w", marker)
			}
			if kind == "exhausted" {
				reason, stop, marker = singleton.FailureExhausted, singleton.ErrRetriesExhausted, errBench
			}
			if kind == "timed_out" {
				reason, stop = singleton.FailureTimedOut, context.DeadlineExceeded
				options = append(options, singleton.WithInitializationTimeout(time.Millisecond))
			}
			for b.Loop() {
				attempts := 0
				provider := singleton.MustNew(func(ctx context.Context) (int, error) {
					attempts++
					if kind == "timed_out" {
						<-ctx.Done()
						return 99, errBench
					}
					return 99, marker
				}, options...)
				value, err := provider.Get(b.Context())
				benchFailure(b, value, err, reason, stop)
				wantAttempts := 1
				if kind == "exhausted" {
					wantAttempts = 3
				}
				if attempts != wantAttempts {
					b.Fatalf("attempts = %d, want %d", attempts, wantAttempts)
				}
			}
		})
	}
}

func BenchmarkRetryObserver(b *testing.B) {
	for _, kind := range []string{"nil", "delivered", "panic"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				attempts, events := 0, 0
				var invalidEvent bool
				var observer func(singleton.RetryEvent)
				if kind != "nil" {
					observer = func(event singleton.RetryEvent) {
						events++
						invalidEvent = invalidEvent || event.Attempt != uint(events) ||
							event.Err != errBench ||
							event.NextDelay <= 0
						if kind == "panic" {
							panic(errBench)
						}
					}
				}
				var factory singleton.Factory[int] = func(context.Context) (int, error) {
					attempts++
					if attempts < 3 {
						return 99, errBench
					}
					return 42, nil
				}
				var provider singleton.Interface[int] = singleton.MustNew(factory,
					append(benchOptions(), singleton.WithRetryObserver(observer))...)
				value, err := provider.Get(b.Context())
				wantEvents := 2
				if kind == "nil" {
					wantEvents = 0
				}
				if value != 42 || err != nil || attempts != 3 || events != wantEvents ||
					invalidEvent {
					b.Fatalf("Get() = (%d, %v), attempts = %d, events = %d, invalid event = %t",
						value, err, attempts, events, invalidEvent)
				}
			}
		})
	}
}

// benchBlockedProvider holds shared initialization open until finish is called.
// Waiting for started makes cancellation and Reset scenarios deterministic.
func benchBlockedProvider(b *testing.B) (*singleton.Provider[int], func()) {
	b.Helper()
	// Cleanup runs after b.Context is canceled; retain an independent wait.
	completionCtx := context.WithoutCancel(b.Context())
	started, release := make(chan struct{}), make(chan struct{})
	var attempts atomic.Uint32
	provider := singleton.MustNew(func(context.Context) (int, error) {
		attempts.Add(1)
		close(started)
		<-release
		return 42, nil
	}, singleton.WithInitializationTimeout(0))
	finish := func() {
		close(release)
		value, err := provider.Get(completionCtx)
		if value != 42 || err != nil || attempts.Load() != 1 {
			b.Fatalf("shared initialization = (%d, %v), attempts = %d", value, err, attempts.Load())
		}
	}
	ctx, cancel := context.WithCancel(b.Context())
	cancel()
	_, err := provider.Get(ctx)
	<-started
	if !errors.Is(err, context.Canceled) {
		finish()
		b.Fatalf("starting caller = %v, want cancellation", err)
	}
	return provider, finish
}

// BenchmarkGetCallerCancellation measures abandoning a shared initialization.
// Only the live_deadline scenario includes waiting for a timer to expire.
func BenchmarkGetCallerCancellation(b *testing.B) {
	for _, kind := range []string{"canceled", "expired_deadline", "custom_cause", "live_deadline"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			provider, finish := benchBlockedProvider(b)
			b.Cleanup(finish)
			ctx, cancel := context.WithCancelCause(b.Context())
			cancel(context.Canceled)
			want := error(context.Canceled)
			if kind == "custom_cause" {
				ctx, cancel = context.WithCancelCause(b.Context())
				cancel(errBench)
				want = errBench
			}
			if kind == "expired_deadline" {
				deadlineCtx, deadlineCancel := context.WithDeadline(
					b.Context(),
					time.Now().Add(-time.Second),
				)
				defer deadlineCancel()
				ctx, want = deadlineCtx, context.DeadlineExceeded
			}
			for b.Loop() {
				if kind == "live_deadline" {
					b.StopTimer()
					deadlineCtx, deadlineCancel := context.WithTimeout(
						b.Context(),
						time.Millisecond,
					)
					b.StartTimer()
					value, err := provider.Get(deadlineCtx)
					b.StopTimer()
					deadlineCancel()
					checkBenchCancellation(b, value, err, context.DeadlineExceeded)
					b.StartTimer()
				} else {
					value, err := provider.Get(ctx)
					checkBenchCancellation(b, value, err, want)
				}
			}
		})
	}
}

func checkBenchCancellation(b *testing.B, value int, err, want error) {
	b.Helper()
	_, initializationFailure := errors.AsType[*singleton.InitError](err)
	if value != 0 || !errors.Is(err, want) || initializationFailure {
		b.Fatalf("Get() = (%d, %v), want caller cause %v", value, err, want)
	}
}

// BenchmarkFirstGetCanceled measures construction and an already-canceled
// first caller; completion of the continuing initialization is untimed.
func BenchmarkFirstGetCanceled(b *testing.B) {
	b.ReportAllocs()
	ctx, cancel := context.WithCancelCause(b.Context())
	cancel(errBench)
	for b.Loop() {
		release := make(chan struct{})
		provider := singleton.MustNew(func(context.Context) (int, error) {
			<-release
			return 42, nil
		}, singleton.WithInitializationTimeout(0))
		value, err := provider.Get(ctx)
		b.StopTimer()
		close(release)
		got, completionErr := provider.Get(b.Context())
		checkBenchCancellation(b, value, err, errBench)
		if got != 42 || completionErr != nil {
			b.Fatalf("Get() after cancellation = (%d, %v)", got, completionErr)
		}
		b.StartTimer()
	}
}

func BenchmarkGetPanic(b *testing.B) {
	b.Run("first_construction_and_get", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			provider := singleton.MustNew(func(context.Context) (int, error) { panic(errBench) })
			benchPanic(b, func() { _, _ = provider.Get(b.Context()) }, errBench)
		}
	})
	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		attempts := 0
		provider := singleton.MustNew(func(context.Context) (int, error) {
			attempts++
			panic(errBench)
		})
		benchPanic(b, func() { _, _ = provider.Get(b.Context()) }, errBench)
		for b.Loop() {
			benchPanic(b, func() { _, _ = provider.Get(b.Context()) }, errBench)
		}
		if attempts != 1 {
			b.Fatalf("cached panic reran factory %d times", attempts)
		}
	})
	for _, kind := range []string{"nil_context", "zero_provider"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			provider := singleton.MustNew(successfulFactory)
			ctx := b.Context()
			want := "singleton: nil context"
			if kind == "nil_context" {
				ctx = nil
			} else {
				provider = new(singleton.Provider[int])
				want = "singleton: Provider must be created with New or MustNew"
			}
			for b.Loop() {
				benchPanic(b, func() { _, _ = provider.Get(ctx) }, want)
			}
		})
	}
}

// BenchmarkReset measures Reset alone, preparing failed states while untimed.
func BenchmarkReset(b *testing.B) {
	for _, kind := range []string{"failed", "panicked"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			attempts := 0
			provider := singleton.MustNew(func(context.Context) (int, error) {
				attempts++
				if kind == "panicked" {
					panic(errBench)
				}
				return 99, singleton.Permanent(errBench)
			})
			for b.Loop() {
				b.StopTimer()
				benchPrepareFailure(b, provider, kind)
				b.StartTimer()
				provider.Reset()
			}
			// One more initialization verifies the final Reset discarded its state.
			benchPrepareFailure(b, provider, kind)
			if attempts != b.N+1 {
				b.Fatalf("attempts = %d, want %d", attempts, b.N+1)
			}
		})
	}
	for _, kind := range []string{"zero", "before_get", "successful", "in_flight"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			provider := new(singleton.Provider[int])
			var attempts atomic.Uint32
			if kind == "in_flight" {
				var finish func()
				provider, finish = benchBlockedProvider(b)
				b.Cleanup(finish)
			} else if kind != "zero" {
				provider = singleton.MustNew(func(context.Context) (int, error) {
					attempts.Add(1)
					return 42, nil
				})
				if kind == "successful" {
					checkBenchConstruction(b, provider, nil, nil)
				}
			}
			for b.Loop() {
				provider.Reset()
			}
			if kind == "before_get" && attempts.Load() != 0 {
				b.Fatal("Reset initialized the provider")
			}
			if kind == "successful" || kind == "before_get" {
				checkBenchConstruction(b, provider, nil, nil)
				if attempts.Load() != 1 {
					b.Fatal("Reset discarded a successful initialization")
				}
			}
		})
	}
}

func benchPrepareFailure(b *testing.B, provider *singleton.Provider[int], kind string) {
	b.Helper()
	if kind == "panicked" {
		benchPanic(b, func() { _, _ = provider.Get(b.Context()) }, errBench)
		return
	}
	value, err := provider.Get(b.Context())
	benchFailure(b, value, err, singleton.FailurePermanent, singleton.ErrPermanent)
}

// BenchmarkResetAndRecover measures resetting a failed/panicked initialization
// and initializing successfully; construction and initial failure are untimed.
func BenchmarkResetAndRecover(b *testing.B) {
	for _, kind := range []string{"failed", "panicked"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				attempts := 0
				var provider singleton.Interface[int] = singleton.MustNew(func(context.Context) (int, error) {
					attempts++
					if attempts == 1 {
						if kind == "panicked" {
							panic(errBench)
						}
						return 99, singleton.Permanent(errBench)
					}
					return 42, nil
				})
				if kind == "panicked" {
					benchPanic(b, func() { _, _ = provider.Get(b.Context()) }, errBench)
				} else {
					value, err := provider.Get(b.Context())
					benchFailure(b, value, err, singleton.FailurePermanent, singleton.ErrPermanent)
				}
				b.StartTimer()
				provider.Reset()
				value, err := provider.Get(b.Context())
				b.StopTimer()
				if value != 42 || err != nil || attempts != 2 {
					b.Fatalf("Get() after Reset = (%d, %v), attempts = %d", value, err, attempts)
				}
				b.StartTimer()
			}
		})
	}
}
