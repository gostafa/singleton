package application_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mostafakhairy0305-dot/singleton/internal/application"
	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

var (
	errInit    = errors.New("initialization failed")
	errGaveUp  = errors.New("caller gave up")
	errNoRetry = errors.New("no retries here")
)

// onceRetrier runs the operation exactly once, so a test controls the outcome
// entirely through its factory.
type onceRetrier[T any] struct{}

func (onceRetrier[T]) Do(ctx context.Context, op ports.Operation[T]) (T, error) {
	return op(ctx)
}

// failingRetrier never runs the operation, standing in for a policy that gave
// up before the first attempt could report anything.
type failingRetrier[T any] struct{}

func (failingRetrier[T]) Do(context.Context, ports.Operation[T]) (T, error) {
	var zero T

	return zero, errNoRetry
}

func recoveredValue(t *testing.T, call func()) any {
	t.Helper()

	var recovered any

	func() {
		defer func() { recovered = recover() }()

		call()
	}()

	if recovered == nil {
		t.Fatal("the call returned normally, want a panic")
	}

	return recovered
}

func TestGetInitializesOnceAndCachesTheValue(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		calls.Add(1)

		return 42, nil
	}, onceRetrier[int]{})

	for range 3 {
		got, err := provider.Get(context.Background())
		if err != nil {
			t.Fatalf("Get() error = %v, want nil", err)
		}

		if got != 42 {
			t.Errorf("Get() = %d, want 42", got)
		}
	}

	if calls.Load() != 1 {
		t.Errorf("the factory ran %d times, want 1", calls.Load())
	}
}

func TestGetCachesTheFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		calls.Add(1)

		return 9, errInit
	}, onceRetrier[int]{})

	for range 2 {
		got, err := provider.Get(context.Background())
		if !errors.Is(err, errInit) {
			t.Fatalf("Get() error = %v, want %v", err, errInit)
		}

		if got != 0 {
			t.Errorf("Get() = %d, want the zero value on failure", got)
		}
	}

	if calls.Load() != 1 {
		t.Errorf("the factory ran %d times, want 1", calls.Load())
	}
}

func TestGetSurfacesARetrierFailureWithoutRunningTheFactory(t *testing.T) {
	t.Parallel()

	provider := application.NewProvider(func(context.Context) (int, error) {
		t.Error("the factory ran, want the retrier to fail first")

		return 0, nil
	}, failingRetrier[int]{})

	_, err := provider.Get(context.Background())
	if !errors.Is(err, errNoRetry) {
		t.Errorf("Get() error = %v, want %v", err, errNoRetry)
	}
}

func TestGetRePanicsWithTheFactoryPanic(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		calls.Add(1)

		panic("factory exploded")
	}, onceRetrier[int]{})

	// The first Get re-panics from the settling state, the second from the
	// cached one.
	for range 2 {
		got := recoveredValue(t, func() { _, _ = provider.Get(context.Background()) })
		if got != "factory exploded" {
			t.Errorf("recovered %v, want %q", got, "factory exploded")
		}
	}

	if calls.Load() != 1 {
		t.Errorf("the factory ran %d times, want 1", calls.Load())
	}
}

func TestGetPanicsOnANilContext(t *testing.T) {
	t.Parallel()

	provider := application.NewProvider(func(context.Context) (int, error) {
		return 1, nil
	}, onceRetrier[int]{})

	var nilCtx context.Context

	got := recoveredValue(t, func() { _, _ = provider.Get(nilCtx) })
	if got != "singleton: nil context" {
		t.Errorf("recovered %v, want %q", got, "singleton: nil context")
	}
}

func TestGetPanicsOnTheZeroProvider(t *testing.T) {
	t.Parallel()

	var provider application.Provider[int]

	const want = "singleton: Provider must be created with New or MustNew"

	got := recoveredValue(t, func() { _, _ = provider.Get(context.Background()) })
	if got != want {
		t.Errorf("recovered %v, want %q", got, want)
	}
}

func TestGetStopsWaitingWhenTheCallerContextEnds(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)

	provider := application.NewProvider(func(context.Context) (int, error) {
		<-release

		return 7, nil
	}, onceRetrier[int]{})

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errGaveUp)

	got, err := provider.Get(ctx)
	if got != 0 {
		t.Errorf("Get() = %d, want the zero value", got)
	}

	if !errors.Is(err, errGaveUp) {
		t.Errorf("Get() error = %v, want it to wrap %v", err, errGaveUp)
	}

	// The abandoned wait must not be mistaken for an initialization failure.
	if errors.Is(err, errInit) {
		t.Error("the caller's error reports an initialization failure")
	}
}

func TestResetBeforeTheFirstGetDoesNothing(t *testing.T) {
	t.Parallel()

	provider := application.NewProvider(func(context.Context) (int, error) {
		return 3, nil
	}, onceRetrier[int]{})

	provider.Reset()

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got != 3 {
		t.Errorf("Get() = %d, want 3", got)
	}
}

func TestResetDiscardsAFailedInitialization(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		if calls.Add(1) == 1 {
			return 0, errInit
		}

		return 5, nil
	}, onceRetrier[int]{})

	_, err := provider.Get(context.Background())
	if !errors.Is(err, errInit) {
		t.Fatalf("Get() error = %v, want %v", err, errInit)
	}

	provider.Reset()

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() after Reset error = %v, want nil", err)
	}

	if got != 5 {
		t.Errorf("Get() after Reset = %d, want 5", got)
	}
}

func TestResetDiscardsAPanickedInitialization(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		if calls.Add(1) == 1 {
			panic("factory exploded")
		}

		return 5, nil
	}, onceRetrier[int]{})

	_ = recoveredValue(t, func() { _, _ = provider.Get(context.Background()) })

	provider.Reset()

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() after Reset error = %v, want nil", err)
	}

	if got != 5 {
		t.Errorf("Get() after Reset = %d, want 5", got)
	}
}

func TestResetKeepsASuccessfulValue(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		return int(calls.Add(1)), nil
	}, onceRetrier[int]{})

	_, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	provider.Reset()

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() after Reset error = %v, want nil", err)
	}

	if got != 1 {
		t.Errorf("Get() after Reset = %d, want the original 1", got)
	}
}

func TestResetLeavesAnInFlightInitializationAlone(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	entered := make(chan struct{})
	release := make(chan struct{})

	provider := application.NewProvider(func(context.Context) (int, error) {
		calls.Add(1)
		close(entered)
		<-release

		return 7, nil
	}, onceRetrier[int]{})

	// Start initialization without waiting for it: an already-cancelled caller
	// abandons its wait the instant the state exists.
	abandoned, cancel := context.WithCancel(context.Background())
	cancel()

	_, abandonedErr := provider.Get(abandoned)
	if abandonedErr == nil {
		t.Fatal("Get() with a cancelled context returned no error")
	}

	<-entered

	provider.Reset()

	close(release)

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got != 7 {
		t.Errorf("Get() = %d, want 7", got)
	}

	if calls.Load() != 1 {
		t.Errorf("the factory ran %d times, want 1", calls.Load())
	}
}

func TestConcurrentGetsShareOneInitialization(t *testing.T) {
	t.Parallel()

	const callers = 64

	var calls atomic.Int64

	provider := application.NewProvider(func(context.Context) (int, error) {
		calls.Add(1)

		return 21, nil
	}, onceRetrier[int]{})

	start := make(chan struct{})

	var group sync.WaitGroup

	group.Add(callers)

	for range callers {
		go func() {
			defer group.Done()

			<-start

			got, err := provider.Get(context.Background())
			if err != nil || got != 21 {
				t.Errorf("Get() = (%d, %v), want (21, nil)", got, err)
			}
		}()
	}

	close(start)
	group.Wait()

	if calls.Load() != 1 {
		t.Errorf("the factory ran %d times, want 1", calls.Load())
	}
}
