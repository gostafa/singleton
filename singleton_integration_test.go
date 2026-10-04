package singleton_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/mostafakhairy0305-dot/singleton"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

var errFactory = errors.New("factory failed")

var (
	errBoom  = errors.New("boom")
	errFatal = errors.New("fatal")
)

func requireInitError(t *testing.T, err error) *singleton.InitError {
	t.Helper()

	return asInitError(t, err)
}

func quickOptions() []singleton.Option {
	return []singleton.Option{
		singleton.WithMaxAttempts(3),
		singleton.WithInitializationTimeout(time.Second),
		singleton.WithRetryInterval(time.Millisecond, 2*time.Millisecond),
	}
}

func successfulFactory(context.Context) (int, error) { return 42, nil }

func asInitError(t *testing.T, err error) *singleton.InitError {
	t.Helper()

	var initErr *singleton.InitError
	if !errors.As(err, &initErr) {
		t.Fatalf("error = %v, want *singleton.InitError", err)
	}

	return initErr
}

func assertEqual(t *testing.T, label string, got, want any) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func assertErrorIs(t *testing.T, err, want error) {
	t.Helper()

	if !errors.Is(err, want) {
		t.Errorf("error = %v, want it to wrap %v", err, want)
	}
}

func TestPublicTypesBelongToSingleton(t *testing.T) {
	t.Parallel()

	types := []reflect.Type{
		reflect.TypeFor[singleton.Provider[int]](),
		reflect.TypeFor[singleton.InitError](),
		reflect.TypeFor[singleton.FailureReason](),
		reflect.TypeFor[singleton.RetryEvent](),
		reflect.TypeFor[singleton.PermanentError](),
		reflect.TypeFor[singleton.Option](),
		reflect.TypeFor[singleton.Interface[int]](),
	}

	for _, typ := range types {
		if typ.PkgPath() != "github.com/mostafakhairy0305-dot/singleton" {
			t.Errorf("%s belongs to %s, want the public package", typ, typ.PkgPath())
		}
	}

	assertEqual(t, "Factory alias", reflect.TypeFor[singleton.Factory[int]](),
		reflect.TypeFor[func(context.Context) (int, error)]())

	var provider singleton.Interface[int] = singleton.MustNew(successfulFactory)

	got, err := provider.Get(context.Background())
	if got != 42 || err != nil {
		t.Errorf("Get() = (%d, %v), want (42, nil)", got, err)
	}
}

func TestPublicFailureReasons(t *testing.T) {
	t.Parallel()

	tests := map[singleton.FailureReason]struct {
		value uint8
		name  string
	}{
		singleton.FailurePermanent: {value: 1, name: "permanent failure"},
		singleton.FailureExhausted: {value: 2, name: "retries exhausted"},
		singleton.FailureTimedOut:  {value: 3, name: "initialization timed out"},
		singleton.FailureCanceled:  {value: 4, name: "initialization canceled"},
		0:                          {value: 0, name: "initialization failed"},
		255:                        {value: 255, name: "initialization failed"},
	}

	for reason, test := range tests {
		if uint8(reason) != test.value || reason.String() != test.name {
			t.Errorf("reason %d = %q, want %d / %q", reason, reason, test.value, test.name)
		}
	}
}

func TestPublicConstructionErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		factory singleton.Factory[int]
		options []singleton.Option
		want    error
	}{
		"nil factory": {factory: nil, options: nil, want: singleton.ErrNilFactory},
		"zero option": {
			factory: successfulFactory,
			options: []singleton.Option{{}},
			want:    singleton.ErrInvalidOption,
		},
		"zero attempts": {
			factory: successfulFactory,
			options: []singleton.Option{singleton.WithMaxAttempts(0)},
			want:    singleton.ErrZeroMaxAttempts,
		},
		"negative timeout": {
			factory: successfulFactory,
			options: []singleton.Option{singleton.WithInitializationTimeout(-time.Second)},
			want:    singleton.ErrNegativeTimeout,
		},
		"nonpositive interval": {
			factory: successfulFactory,
			options: []singleton.Option{singleton.WithRetryInterval(-time.Second, time.Second)},
			want:    singleton.ErrZeroInitialInterval,
		},
		"maximum below initial": {
			factory: successfulFactory,
			options: []singleton.Option{singleton.WithRetryInterval(time.Second, time.Millisecond)},
			want:    singleton.ErrMaxIntervalBelowInitial,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			provider, err := singleton.New(test.factory, test.options...)
			if provider != nil || !errors.Is(err, test.want) {
				t.Errorf("New() = (%v, %v), want (nil, %v)", provider, err, test.want)
			}
		})
	}
}

func TestPublicPermanentError(t *testing.T) {
	t.Parallel()

	if singleton.Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must return nil")
	}

	err := singleton.Permanent(errFactory)

	var permanent *singleton.PermanentError
	if !errors.As(err, &permanent) {
		t.Fatalf("Permanent() = %T, want *singleton.PermanentError", err)
	}

	assertEqual(t, "PermanentError.Err", permanent.Err, errFactory)
	assertEqual(t, "PermanentError.Error()", permanent.Error(), errFactory.Error())
	assertEqual(t, "PermanentError.Unwrap()", errors.Unwrap(permanent), errFactory)
	assertErrorIs(t, permanent, errFactory)
}

func TestPublicPermanentErrorsStopRetrying(t *testing.T) {
	t.Parallel()

	markers := map[string]error{
		"helper": singleton.Permanent(errFactory),
		"literal": &singleton.PermanentError{
			Err: errFactory,
		},
		"wrapped": fmt.Errorf("configuration: %w", singleton.Permanent(errFactory)),
	}

	for name, marker := range markers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var attempts atomic.Uint32

			provider := singleton.MustNew(func(context.Context) (int, error) {
				attempts.Add(1)

				return 99, marker
			}, quickOptions()...)

			value, err := provider.Get(context.Background())
			initErr := asInitError(t, err)

			assertEqual(t, "value", value, 0)
			assertEqual(t, "attempts", attempts.Load(), uint32(1))
			assertEqual(t, "reason", initErr.Reason, singleton.FailurePermanent)
			assertEqual(t, "factory error", initErr.Err, errFactory)
			assertErrorIs(t, err, singleton.ErrPermanent)
			assertErrorIs(t, err, errFactory)
		})
	}
}

func TestPublicInitErrorIsCachedAndReset(t *testing.T) {
	t.Parallel()

	var attempts atomic.Uint32

	provider := singleton.MustNew(func(context.Context) (int, error) {
		if attempts.Add(1) <= 3 {
			return 99, errFactory
		}

		return 42, nil
	}, quickOptions()...)

	value, err := provider.Get(context.Background())
	initErr := asInitError(t, err)

	assertEqual(t, "value", value, 0)
	assertEqual(t, "factory error", initErr.Err, errFactory)
	assertEqual(t, "reason", initErr.Reason, singleton.FailureExhausted)
	assertErrorIs(t, err, singleton.ErrRetriesExhausted)
	assertErrorIs(t, err, errFactory)

	_, cached := provider.Get(context.Background())

	assertEqual(t, "cached error identity", cached, err)
	assertEqual(t, "attempts", attempts.Load(), uint32(3))

	provider.Reset()

	got, err := provider.Get(context.Background())

	assertEqual(t, "value after Reset", got, 42)
	assertEqual(t, "error after Reset", err, nil)

	provider.Reset()
	_, _ = provider.Get(context.Background())

	assertEqual(t, "attempts after resetting success", attempts.Load(), uint32(4))
}

func TestPublicInitializationTimeout(t *testing.T) {
	t.Parallel()

	provider := singleton.MustNew(func(ctx context.Context) (int, error) {
		<-ctx.Done()

		return 99, errFactory
	}, singleton.WithInitializationTimeout(20*time.Millisecond))

	value, err := provider.Get(context.Background())
	initErr := asInitError(t, err)

	assertEqual(t, "value", value, 0)
	assertEqual(t, "reason", initErr.Reason, singleton.FailureTimedOut)
	assertEqual(t, "factory error", initErr.Err, errFactory)
	assertErrorIs(t, err, context.DeadlineExceeded)
	assertErrorIs(t, err, errFactory)
}

func TestPublicReasonDoesNotComeFromFactoryError(t *testing.T) {
	t.Parallel()

	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, context.DeadlineExceeded
	}, quickOptions()...)

	_, err := provider.Get(context.Background())
	initErr := asInitError(t, err)

	if initErr.Reason != singleton.FailureExhausted ||
		!errors.Is(
			err,
			singleton.ErrRetriesExhausted,
		) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Get() = %v, want exhausted retries wrapping the factory's deadline error", err)
	}
}

func TestPublicRetryEvents(t *testing.T) {
	t.Parallel()

	var events []singleton.RetryEvent

	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, errFactory
	}, append(quickOptions(), singleton.WithRetryObserver(func(event singleton.RetryEvent) {
		events = append(events, event)
	}))...)

	_, err := provider.Get(context.Background())
	if !errors.Is(err, singleton.ErrRetriesExhausted) || len(events) != 2 {
		t.Fatalf("Get() = %v, events = %v, want exhaustion and 2 events", err, events)
	}

	for index, event := range events {
		assertEqual(t, "attempt", event.Attempt, uint(index+1))
		assertEqual(t, "factory error", event.Err, errFactory)

		if event.NextDelay <= 0 {
			t.Errorf(
				"event %d = %+v, want numbered retry with the original error and positive delay",
				index,
				event,
			)
		}
	}
}

func TestPublicObserverPanicIsDiscarded(t *testing.T) {
	t.Parallel()

	var events atomic.Uint32

	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, errFactory
	}, append(quickOptions(), singleton.WithRetryObserver(func(singleton.RetryEvent) {
		events.Add(1)

		panic("observer panic")
	}))...)

	_, err := provider.Get(context.Background())
	if !errors.Is(err, singleton.ErrRetriesExhausted) || events.Load() != 2 {
		t.Errorf("Get() = %v, events = %d, want exhausted retries and 2 events", err, events.Load())
	}
}

func TestPublicInitErrorLiteral(t *testing.T) {
	t.Parallel()

	initErr := &singleton.InitError{Reason: singleton.FailureExhausted, Err: errFactory}
	if initErr.Error() != "singleton: retries exhausted: factory failed" ||
		!errors.Is(initErr, errFactory) || errors.Unwrap(initErr) != nil {
		t.Errorf("InitError literal = %v, unwrap = %v", initErr, initErr.Unwrap())
	}

	empty := new(singleton.InitError)
	if empty.Unwrap() != nil {
		t.Errorf("empty InitError unwrap = %v, want nil", empty.Unwrap())
	}
}

func TestPublicCallerCancellationIsIsolated(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	defer close(release)

	provider := singleton.MustNew(func(ctx context.Context) (int, error) {
		<-release

		return 42, ctx.Err()
	})

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errFactory)

	value, err := provider.Get(ctx)
	if value != 0 || !errors.Is(err, errFactory) {
		t.Errorf("Get() = (%d, %v), want caller cancellation", value, err)
	}

	if _, ok := errors.AsType[*singleton.InitError](err); ok {
		t.Error("caller cancellation was converted to an initialization error")
	}

	release <- struct{}{}

	got, err := provider.Get(context.Background())

	assertEqual(t, "shared value after caller cancellation", got, 42)
	assertEqual(t, "shared error after caller cancellation", err, nil)
}

func assertPanic(t *testing.T, call func(), want any) {
	t.Helper()

	defer func() {
		if got := recover(); got != want {
			t.Errorf("panic = %v, want %v", got, want)
		}
	}()

	call()
	t.Error("call did not panic")
}

func TestPublicProviderMisuse(t *testing.T) {
	t.Parallel()

	var provider singleton.Provider[int]

	provider.Reset()

	var nilContext context.Context

	assertPanic(t, func() { _, _ = provider.Get(nilContext) }, "singleton: nil context")
	assertPanic(t, func() { _, _ = provider.Get(context.Background()) },
		"singleton: Provider must be created with New or MustNew")
}

func TestPublicFactoryPanicIsCachedUntilReset(t *testing.T) {
	t.Parallel()

	var attempts atomic.Uint32

	provider := singleton.MustNew(func(context.Context) (int, error) {
		if attempts.Add(1) == 1 {
			panic(errFactory)
		}

		return 42, nil
	})

	for range 2 {
		assertPanic(t, func() { _, _ = provider.Get(context.Background()) }, errFactory)
	}

	if attempts.Load() != 1 {
		t.Error("factory panic was not cached")
	}

	provider.Reset()

	got, err := provider.Get(context.Background())

	assertEqual(t, "value after resetting panic", got, 42)
	assertEqual(t, "error after resetting panic", err, nil)
}

func okFactory(context.Context) (int, error) { return 1, nil }

// fastOptions keep the retry budget small and its delays short, so a test that
// drives initialization to failure costs milliseconds.
func fastOptions() []singleton.Option {
	return []singleton.Option{
		singleton.WithMaxAttempts(3),
		singleton.WithInitializationTimeout(2 * time.Second),
		singleton.WithRetryInterval(time.Millisecond, 2*time.Millisecond),
	}
}

func TestNewRejectsInvalidConstruction(t *testing.T) {
	t.Parallel()

	var zeroOption singleton.Option

	tests := map[string]struct {
		factory singleton.Factory[int]
		options []singleton.Option
		want    error
	}{
		"nil factory": {factory: nil, options: nil, want: singleton.ErrNilFactory},
		"zero option": {
			factory: okFactory,
			options: []singleton.Option{zeroOption},
			want:    singleton.ErrInvalidOption,
		},
		"zero max attempts": {
			factory: okFactory,
			options: []singleton.Option{singleton.WithMaxAttempts(0)},
			want:    singleton.ErrZeroMaxAttempts,
		},
		"negative timeout": {
			factory: okFactory,
			options: []singleton.Option{singleton.WithInitializationTimeout(-time.Second)},
			want:    singleton.ErrNegativeTimeout,
		},
		"zero initial interval": {
			factory: okFactory,
			options: []singleton.Option{singleton.WithRetryInterval(0, time.Second)},
			want:    singleton.ErrZeroInitialInterval,
		},
		"maximum below initial": {
			factory: okFactory,
			options: []singleton.Option{singleton.WithRetryInterval(2*time.Second, time.Second)},
			want:    singleton.ErrMaxIntervalBelowInitial,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			provider, err := singleton.New(test.factory, test.options...)
			if provider != nil {
				t.Errorf("New() provider = %v, want nil", provider)
			}

			if !errors.Is(err, test.want) {
				t.Errorf("New() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewAppliesEveryOption(t *testing.T) {
	t.Parallel()

	options := append(
		fastOptions(),
		singleton.WithRetryObserver(func(singleton.RetryEvent) {}),
		// A nil observer is allowed and simply disables the callback.
		singleton.WithRetryObserver(nil),
		// A zero timeout disables the deadline.
		singleton.WithInitializationTimeout(0),
	)

	provider, err := singleton.New(okFactory, options...)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got != 1 {
		t.Errorf("Get() = %d, want 1", got)
	}
}

func TestMustNewReturnsAProvider(t *testing.T) {
	t.Parallel()

	provider := singleton.MustNew(okFactory, fastOptions()...)

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v, want nil", err)
	}

	if got != 1 {
		t.Errorf("Get() = %d, want 1", got)
	}
}

func TestPermanentReturnsNilForNil(t *testing.T) {
	t.Parallel()

	got := singleton.Permanent(nil)
	if got != nil {
		t.Errorf("Permanent(nil) = %v, want nil", got)
	}
}

func TestMustNewPanicsOnInvalidConstruction(t *testing.T) {
	t.Parallel()

	var recovered any

	func() {
		defer func() { recovered = recover() }()

		_ = singleton.MustNew[int](nil)
	}()

	err, ok := recovered.(error)
	if !ok {
		t.Fatalf("recovered %v, want an error", recovered)
	}

	if !errors.Is(err, singleton.ErrNilFactory) {
		t.Errorf("recovered error = %v, want %v", err, singleton.ErrNilFactory)
	}
}

func TestPermanentStopsRetryingAtTheFirstAttempt(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	provider := singleton.MustNew(func(context.Context) (int, error) {
		calls.Add(1)

		return 0, singleton.Permanent(errFatal)
	}, fastOptions()...)

	_, err := provider.Get(context.Background())

	initErr := requireInitError(t, err)
	if initErr.Reason != singleton.FailurePermanent {
		t.Errorf("Reason = %v, want %v", initErr.Reason, singleton.FailurePermanent)
	}

	if !errors.Is(err, errFatal) {
		t.Errorf("Get() error = %v, want it to wrap %v", err, errFatal)
	}

	if calls.Load() != 1 {
		t.Errorf("the factory ran %d times, want 1", calls.Load())
	}
}

func TestGetReportsAnExhaustedBudget(t *testing.T) {
	t.Parallel()

	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, errBoom
	}, fastOptions()...)

	_, err := provider.Get(context.Background())

	initErr := requireInitError(t, err)
	if initErr.Reason != singleton.FailureExhausted {
		t.Errorf("Reason = %v, want %v", initErr.Reason, singleton.FailureExhausted)
	}

	if !errors.Is(err, errBoom) {
		t.Errorf("Get() error = %v, want it to wrap %v", err, errBoom)
	}
}

func TestResetStartsANewInitialization(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64

	// Depending on the exported interface is how a consumer substitutes a fake.
	var provider singleton.Interface[int] = singleton.MustNew(func(context.Context) (int, error) {
		if calls.Add(1) <= 3 {
			return 0, errBoom
		}

		return 8, nil
	}, fastOptions()...)

	_, err := provider.Get(context.Background())
	if err == nil {
		t.Fatal("Get() error = nil, want the exhausted budget")
	}

	provider.Reset()

	got, err := provider.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() after Reset error = %v, want nil", err)
	}

	if got != 8 {
		t.Errorf("Get() after Reset = %d, want 8", got)
	}
}

func TestGetReportsTheInitializationTimeout(t *testing.T) {
	t.Parallel()

	provider := singleton.MustNew(func(ctx context.Context) (int, error) {
		<-ctx.Done()

		return 0, errBoom
	},
		singleton.WithMaxAttempts(5),
		singleton.WithInitializationTimeout(20*time.Millisecond),
		singleton.WithRetryInterval(time.Millisecond, 2*time.Millisecond),
	)

	_, err := provider.Get(context.Background())

	initErr := requireInitError(t, err)
	if initErr.Reason != singleton.FailureTimedOut {
		t.Errorf("Reason = %v, want %v", initErr.Reason, singleton.FailureTimedOut)
	}
}

func TestNewMakesTheRetryObserverPanicSafe(t *testing.T) {
	t.Parallel()

	var events atomic.Int64

	provider := singleton.MustNew(func(context.Context) (int, error) {
		return 0, errBoom
	}, append(fastOptions(), singleton.WithRetryObserver(func(singleton.RetryEvent) {
		events.Add(1)

		panic("observer exploded")
	}))...)

	// A panicking observer must not become the singleton's result.
	_, err := provider.Get(context.Background())

	initErr := requireInitError(t, err)
	if initErr.Reason != singleton.FailureExhausted {
		t.Errorf("Reason = %v, want %v", initErr.Reason, singleton.FailureExhausted)
	}

	if events.Load() != 2 {
		t.Errorf("the observer saw %d events, want 2", events.Load())
	}
}
