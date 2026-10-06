// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"fmt"

	"github.com/gostafa/singleton/internal/ports"
)

// NewProvider wires a factory to the retry policy that will drive it.
func NewProvider[Value any](
	factory ports.Operation[Value],
	retrier ports.Retrier[Value],
) *Provider[Value] {
	provider := new(lifecycle[Value])

	provider.factory = factory
	provider.retrier = retrier

	return &Provider[Value]{lifecycle: provider}
}

// Get waits for and returns the shared value, starting initialization on the
// first call and blocking until it settles.
//
// The caller's context cancels only this caller's wait. It does not cancel the
// shared initialization, so one short-lived request cannot poison the
// singleton for the entire process. A failed initialization is cached and
// returned to every later caller until [Provider.Reset] is called.
//
// Get returns the zero Value with an error wrapping context.Cause(ctx) if the
// caller's context ends before initialization settles, and re-panics with the
// factory's panic value if the factory panicked. It panics if ctx is nil or if
// the Provider is the zero value.
func (provider *Provider[Value]) Get(ctx context.Context) (Value, error) {
	//nolint:wrapcheck // Preserve the cached initialization error.
	return get(ctx, provider.lifecycle)
}

func get[Value any](ctx context.Context, provider *lifecycle[Value]) (Value, error) {
	if ctx == nil {
		//nolint:forbidigo // Preserve the documented panic contract.
		panic("singleton: nil context")
	}

	current := load(provider)
	//nolint:wrapcheck // Preserve the cached initialization error.
	return wait(ctx, current)
}

func wait[Value any](ctx context.Context, currentState *state[Value]) (Value, error) {
	if currentState.settled.Load() {
		//nolint:wrapcheck // Every caller receives the same cached error.
		return result(currentState)
	}

	select {
	case <-currentState.done:
		//nolint:wrapcheck // Every caller receives the same cached error.
		return result(currentState)

	case <-ctx.Done():
		//nolint:wrapcheck // The caller cancellation error already carries its context.
		return abandon(ctx, currentState)
	}
}

// Reset discards a failed or panicked initialization so the next
// [Provider.Get] starts a new one.
//
// It does nothing while initialization is in progress, so callers already
// waiting are never left on a discarded state, and nothing after a success,
// because a live value other goroutines already hold must not be torn down.
func (provider *Provider[Value]) Reset() {
	reset(provider.lifecycle)
}

func reset[Value any](provider *lifecycle[Value]) {
	if provider == nil {
		return
	}

	provider.mu.Lock()
	defer provider.mu.Unlock()

	current := provider.current.Load()
	if canReset(current) {
		provider.current.Store(nil)
	}
}

func load[Value any](provider *lifecycle[Value]) *state[Value] {
	if provider == nil {
		//nolint:forbidigo // Preserve the documented zero-value panic.
		panic(uninitializedProvider)
	}

	if current := provider.current.Load(); current != nil {
		return current
	}

	provider.mu.Lock()
	defer provider.mu.Unlock()

	if current := provider.current.Load(); current != nil {
		return current
	}

	return start(provider)
}

func start[Value any](provider *lifecycle[Value]) *state[Value] {
	if provider.factory == nil {
		//nolint:forbidigo // Preserve the documented panic contract.
		panic(uninitializedProvider)
	}

	current := new(state[Value])

	current.done = make(chan struct{})

	provider.current.Store(current)

	go initialize(provider, current)

	return current
}

func result[Value any](currentState *state[Value]) (Value, error) {
	if currentState.panicked {
		//nolint:forbidigo // Preserve the documented panic contract.
		panic(currentState.panicValue)
	}

	return currentState.value, currentState.err
}

// abandon reports why this caller stopped waiting, unless initialization
// settled in the same instant the caller's context ended.
//
// The error wraps context.Cause(ctx), so a context.WithCancelCause reason
// stays reachable through errors.Is and errors.As. It describes the caller's
// own context rather than the singleton, which keeps initializing.
func abandon[Value any](ctx context.Context, currentState *state[Value]) (Value, error) {
	if currentState.settled.Load() {
		//nolint:wrapcheck // Preserve cached error identity.
		return result(currentState)
	}

	var zero Value

	return zero, fmt.Errorf(
		"singleton: waiting for initialization: %w",
		context.Cause(ctx),
	)
}

func initialize[Value any](provider *lifecycle[Value], current *state[Value]) {
	defer finish(current)

	value, err := provider.retrier.Do(context.Background(), provider.factory)
	if err != nil {
		current.err = err

		return
	}

	current.value = value
}

func finish[Value any](currentState *state[Value]) {
	// revive:disable-next-line:defer finish is directly deferred by initialize.
	if value := recover(); value != nil {
		currentState.panicked = true
		currentState.panicValue = value
	}

	currentState.settled.Store(true)
	close(currentState.done)
}

func canReset[Value any](current *state[Value]) bool {
	if current == nil {
		return false
	}

	if !current.settled.Load() {
		return false
	}

	return current.panicked || current.err != nil
}
