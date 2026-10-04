// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
)

var errFactory = errors.New("factory failed")

func TestRetryObserverSafeReturnsNilForNil(t *testing.T) {
	t.Parallel()

	var observer domain.RetryObserver

	if domain.Safe(observer) != nil {
		t.Error("Safe() on a nil observer = non-nil, want nil")
	}
}

func TestRetryObserverSafeDeliversTheEvent(t *testing.T) {
	t.Parallel()

	var got domain.RetryEvent

	observer := domain.RetryObserver(func(event domain.RetryEvent) { got = event })
	want := domain.RetryEvent{Attempt: 2, Err: errFactory, NextDelay: time.Second}

	domain.Safe(observer)(want)

	if got != want {
		t.Errorf("observed %+v, want %+v", got, want)
	}
}

func TestRetryObserverSafeRecoversFromAPanic(t *testing.T) {
	t.Parallel()

	called := false

	observer := domain.RetryObserver(func(domain.RetryEvent) {
		called = true

		panic("observer exploded")
	})

	domain.Safe(observer)(domain.RetryEvent{Attempt: 1, Err: errFactory, NextDelay: 0})

	if !called {
		t.Error("the observer was never called")
	}
}
