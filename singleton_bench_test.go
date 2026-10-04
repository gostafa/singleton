// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mostafakhairy0305-dot/singleton"
)

var errBench = errors.New("bench failure")

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
	ctx := context.Background()
	provider := singleton.MustNew(successfulFactory)

	_, err := provider.Get(ctx)
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		_, err = provider.Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetInitializedParallel measures the cached hot path under
// concurrent callers.
func BenchmarkGetInitializedParallel(b *testing.B) {
	ctx := context.Background()
	provider := singleton.MustNew(successfulFactory)

	_, err := provider.Get(ctx)
	if err != nil {
		b.Fatal(err)
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, getErr := provider.Get(ctx)
			if getErr != nil {
				b.Error(getErr)

				return
			}
		}
	})
}

// BenchmarkNew measures provider construction with and without options.
func BenchmarkNew(b *testing.B) {
	b.Run("default", func(b *testing.B) {
		for b.Loop() {
			_, err := singleton.New(successfulFactory)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with_options", func(b *testing.B) {
		options := append(
			benchOptions(),
			singleton.WithRetryObserver(func(singleton.RetryEvent) {}),
		)

		for b.Loop() {
			_, err := singleton.New(successfulFactory, options...)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkFirstGet measures a cold start: construct a provider and run its
// first successful initialization.
func BenchmarkFirstGet(b *testing.B) {
	ctx := context.Background()

	for b.Loop() {
		provider := singleton.MustNew(successfulFactory)

		_, err := provider.Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFirstGetConcurrentCallers measures a cold start where several
// goroutines race to read the same uninitialized provider.
func BenchmarkFirstGetConcurrentCallers(b *testing.B) {
	const callers = 8

	ctx := context.Background()
	errs := make(chan error, callers)

	for b.Loop() {
		provider := singleton.MustNew(successfulFactory)

		for range callers {
			go func() {
				_, err := provider.Get(ctx)
				errs <- err
			}()
		}

		for range callers {
			if err := <-errs; err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkPermanentFailure measures an initialization that stops on its
// first attempt because the factory returned a permanent error.
func BenchmarkPermanentFailure(b *testing.B) {
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

		_, err := provider.Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGetCachedFailure measures Get on a provider whose failed
// initialization is cached.
func BenchmarkGetCachedFailure(b *testing.B) {
	ctx := context.Background()
	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, singleton.Permanent(errBench)
	}, benchOptions()...)

	_, err := provider.Get(ctx)
	if err == nil {
		b.Fatal("expected an error")
	}

	for b.Loop() {
		_, err = provider.Get(ctx)
		if err == nil {
			b.Fatal("expected an error")
		}
	}
}

// BenchmarkResetAndReinitialize measures discarding a failed initialization
// with Reset and running a fresh one.
func BenchmarkResetAndReinitialize(b *testing.B) {
	ctx := context.Background()
	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, singleton.Permanent(errBench)
	}, benchOptions()...)

	for b.Loop() {
		provider.Reset()

		_, err := provider.Get(ctx)
		if err == nil {
			b.Fatal("expected an error")
		}
	}
}

// BenchmarkInitErrorInspection measures classifying a returned failure with
// errors.Is, errors.As and Error().
func BenchmarkInitErrorInspection(b *testing.B) {
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
