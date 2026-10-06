// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/gostafa/singleton/internal/adapters/backoffretry"
	"github.com/gostafa/singleton/internal/domain"
)

var (
	errBoom  = errors.New("boom")
	errFatal = errors.New("fatal")
)

// fastConfig keeps the delays short enough that a full retry budget costs
// milliseconds rather than seconds.
func fastConfig(
	attempts uint,
	timeout time.Duration,
	observer domain.RetryObserver,
) backoffretry.Config {
	return backoffretry.Config{
		MaxAttempts:     attempts,
		Timeout:         timeout,
		InitialInterval: time.Millisecond,
		MaxInterval:     2 * time.Millisecond,
		Observer:        observer,
	}
}

func requireInitError(t *testing.T, err error) domain.InitError {
	t.Helper()

	var initErr domain.InitError

	if !errors.As(err, &initErr) {
		t.Fatalf("errors.As(%v, domain.InitError) = false, want true", err)
	}

	return initErr
}

// assertRetryEvents checks that the observer saw one event per retried attempt,
// numbered from one.
func assertRetryEvents(t *testing.T, events []domain.RetryEvent, want int) {
	t.Helper()

	if len(events) != want {
		t.Fatalf("observed %d events, want %d", len(events), want)
	}

	for index, event := range events {
		assertRetryEvent(t, index, event)
	}
}

func assertRetryEvent(t *testing.T, index int, event domain.RetryEvent) {
	t.Helper()

	if event.Attempt != uint(index+1) {
		t.Errorf("event %d Attempt = %d, want %d", index, event.Attempt, index+1)
	}

	if !errors.Is(event.Err, errBoom) {
		t.Errorf("event %d Err = %v, want %v", index, event.Err, errBoom)
	}

	if event.NextDelay <= 0 {
		t.Errorf("event %d NextDelay = %v, want a positive delay", index, event.NextDelay)
	}
}

func TestDoReturnsTheFirstSuccess(t *testing.T) {
	t.Parallel()

	calls := 0

	got, err := backoffretry.New[int](fastConfig(3, time.Second, nil)).
		Do(t.Context(), func(context.Context) (int, error) {
			calls++

			return 42, nil
		})
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}

	if got != 42 {
		t.Errorf("Do() = %d, want 42", got)
	}

	if calls != 1 {
		t.Errorf("the operation ran %d times, want 1", calls)
	}
}

func TestDoRetriesUntilSuccessAndNotifiesTheObserver(t *testing.T) {
	t.Parallel()

	var events []domain.RetryEvent

	// A zero timeout leaves the parent context untouched.
	cfg := fastConfig(5, 0, func(event domain.RetryEvent) {
		events = append(events, event)
	})

	calls := 0

	got, err := backoffretry.New[string](cfg).
		Do(t.Context(), func(context.Context) (string, error) {
			calls++
			if calls < 3 {
				return "", errBoom
			}

			return "ready", nil
		})
	if err != nil {
		t.Fatalf("Do() error = %v, want nil", err)
	}

	if got != "ready" {
		t.Errorf("Do() = %q, want %q", got, "ready")
	}

	if calls != 3 {
		t.Errorf("the operation ran %d times, want 3", calls)
	}

	// The winning attempt reports through Do, not through the observer.
	assertRetryEvents(t, events, 2)
}

func TestDoReportsAnExhaustedAttemptBudget(t *testing.T) {
	t.Parallel()

	calls := 0

	// A nil observer still runs the notify callback, which must return early.
	got, err := backoffretry.New[int](fastConfig(3, time.Second, nil)).
		Do(t.Context(), func(context.Context) (int, error) {
			calls++

			return 7, errBoom
		})

	if got != 0 {
		t.Errorf("Do() = %d, want the zero value on failure", got)
	}

	if calls != 3 {
		t.Errorf("the operation ran %d times, want 3", calls)
	}

	initErr := requireInitError(t, err)
	if initErr.Reason() != domain.FailureExhausted {
		t.Errorf("Reason = %v, want %v", initErr.Reason(), domain.FailureExhausted)
	}

	if !errors.Is(initErr.Err(), errBoom) {
		t.Errorf("Err = %v, want %v", initErr.Err(), errBoom)
	}
}

func TestDoStopsAtAPermanentError(t *testing.T) {
	t.Parallel()

	calls := 0

	_, err := backoffretry.New[int](fastConfig(5, time.Second, nil)).
		Do(t.Context(), func(context.Context) (int, error) {
			calls++

			return 3, fmt.Errorf("dial: %w", backoffretry.Permanent(errFatal))
		})

	if calls != 1 {
		t.Errorf("the operation ran %d times, want 1", calls)
	}

	initErr := requireInitError(t, err)
	if initErr.Reason() != domain.FailurePermanent {
		t.Errorf("Reason = %v, want %v", initErr.Reason(), domain.FailurePermanent)
	}

	if !errors.Is(initErr.Err(), errFatal) {
		t.Errorf("Err = %v, want %v", initErr.Err(), errFatal)
	}
}

func TestDoReportsItsOwnTimeout(t *testing.T) {
	t.Parallel()

	_, err := backoffretry.New[int](fastConfig(5, 20*time.Millisecond, nil)).
		Do(t.Context(), func(ctx context.Context) (int, error) {
			<-ctx.Done()

			return 0, errBoom
		})

	initErr := requireInitError(t, err)
	if initErr.Reason() != domain.FailureTimedOut {
		t.Errorf("Reason = %v, want %v", initErr.Reason(), domain.FailureTimedOut)
	}
}

func TestDoReportsACancelledParentContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// A zero timeout means the parent context is the only stop condition.
	_, err := backoffretry.New[int](fastConfig(5, 0, nil)).
		Do(ctx, func(ctx context.Context) (int, error) {
			cancel()
			<-ctx.Done()

			return 0, errBoom
		})

	initErr := requireInitError(t, err)
	if initErr.Reason() != domain.FailureCanceled {
		t.Errorf("Reason = %v, want %v", initErr.Reason(), domain.FailureCanceled)
	}
}
