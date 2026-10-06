// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"testing"
	"time"

	"github.com/gostafa/singleton/internal/ports"
)

// lockHandoff is how long a test waits for a goroutine to park on the
// provider's mutex before the test hands it something to find.
const lockHandoff = 50 * time.Millisecond

// onceRetrier runs the operation exactly once, so a test controls the outcome
// entirely through its factory.
type onceRetrier[T any] struct{}

func (onceRetrier[T]) Do(ctx context.Context, op ports.Operation[T]) (T, error) {
	return op(ctx)
}

func TestAbandonReturnsTheResultWhenInitializationWinsTheRace(t *testing.T) {
	t.Parallel()

	current := new(state[int])

	current.done = make(chan struct{})
	current.value = 99
	current.settled.Store(true)
	close(current.done)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got, err := abandon(ctx, current)
	if err != nil {
		t.Fatalf("abandon() error = %v, want nil", err)
	}

	if got != 99 {
		t.Errorf("abandon() = %d, want 99", got)
	}
}

func TestLoadReturnsTheStateStoredWhileItWaitedForTheLock(t *testing.T) {
	t.Parallel()

	provider := NewProvider(func(context.Context) (int, error) {
		t.Error("the factory ran, want load to adopt the stored state")

		return 0, nil
	}, onceRetrier[int]{})

	existing := new(state[int])

	existing.done = make(chan struct{})
	existing.value = 11
	existing.settled.Store(true)
	close(existing.done)

	provider.lifecycle.mu.Lock()

	loaded := make(chan *state[int], 1)

	go func() { loaded <- load(provider.lifecycle) }()

	// The goroutine finds a nil state and parks on the held mutex; only then is
	// there a state for it to discover on the second check.
	time.Sleep(lockHandoff)
	provider.lifecycle.current.Store(existing)
	provider.lifecycle.mu.Unlock()

	if got := <-loaded; got != existing {
		t.Errorf("load() = %p, want the stored state %p", got, existing)
	}
}

func TestResetZeroProviderDoesNothing(t *testing.T) {
	t.Parallel()
	var provider Provider[int]
	provider.Reset()
}

func TestGetRejectsMissingFactory(t *testing.T) {
	t.Parallel()
	provider := NewProvider[int](nil, onceRetrier[int]{})
	defer func() {
		if got := recover(); got != uninitializedProvider {
			t.Errorf("panic = %v, want %q", got, uninitializedProvider)
		}
	}()
	_, _ = provider.Get(t.Context())
}
