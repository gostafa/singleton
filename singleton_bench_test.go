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

	"github.com/gostafa/singleton"
)

var errBench = errors.New("bench failure")

type benchConstructionCase struct {
	name    string
	factory singleton.Factory[int]
	options []singleton.Option
	want    error
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
		{name: "nil_factory", want: singleton.ErrNilFactory},
		{
			name: "zero_option", factory: successfulFactory,
			options: []singleton.Option{{}}, want: singleton.ErrInvalidOption,
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
// Only live_deadline includes context creation and waiting for a timer to expire.
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
					deadlineCtx, deadlineCancel := context.WithTimeout(
						b.Context(),
						time.Millisecond,
					)
					value, err := provider.Get(deadlineCtx)
					deadlineCancel()
					checkBenchCancellation(b, value, err, context.DeadlineExceeded)
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

// BenchmarkReset measures Reset alone on states that do not require reinitialization.
func BenchmarkReset(b *testing.B) {
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

// BenchmarkFailureAndRecover measures construction, failed/panicked initialization,
// Reset, and successful recovery as one complete cycle.
func BenchmarkFailureAndRecover(b *testing.B) {
	for _, kind := range []string{"failed", "panicked"} {
		b.Run(kind, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
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
				provider.Reset()
				value, err := provider.Get(b.Context())
				if value != 42 || err != nil || attempts != 2 {
					b.Fatalf("Get() after Reset = (%d, %v), attempts = %d", value, err, attempts)
				}
			}
		})
	}
}
