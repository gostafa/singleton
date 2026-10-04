package application

import (
	"context"
	"fmt"
	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

// NewProvider wires a factory to the retry policy that will drive it.
func NewProvider[T any](
	factory ports.Operation[T],
	retrier ports.Retrier[T],
) *Provider[T] {
	provider := new(Provider[T])
	provider.factory = factory
	provider.retrier = retrier

	return provider
}

// Get waits for and returns the shared value, starting initialization on the
// first call and blocking until it settles.
//
// The caller's context cancels only this caller's wait. It does not cancel the
// shared initialization, so one short-lived request cannot poison the
// singleton for the entire process. A failed initialization is cached and
// returned to every later caller until [Provider.Reset] is called.
//
// Get returns the zero T with an error wrapping context.Cause(ctx) if the
// caller's context ends before initialization settles, and re-panics with the
// factory's panic value if the factory panicked. It panics if ctx is nil or if
// the Provider is the zero value.
func (p *Provider[T]) Get(ctx context.Context) (T, error) {
	if ctx == nil {
		panic("singleton: nil context")
	}

	current := p.load()

	if current.settled.Load() {
		return current.result()
	}

	select {
	case <-current.done:
		return current.result()

	case <-ctx.Done():
		return current.abandon(ctx)
	}
}

// Reset discards a failed or panicked initialization so the next
// [Provider.Get] starts a new one.
//
// It does nothing while initialization is in progress, so callers already
// waiting are never left on a discarded state, and nothing after a success,
// because a live value other goroutines already hold must not be torn down.
func (p *Provider[T]) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	current := p.current.Load()
	if current == nil {
		return
	}

	if !current.settled.Load() {
		return
	}

	if current.panicked || current.err != nil {
		p.current.Store(nil)
	}
}

func (p *Provider[T]) load() *state[T] {
	if current := p.current.Load(); current != nil {
		return current
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if current := p.current.Load(); current != nil {
		return current
	}

	if p.factory == nil {
		panic("singleton: Provider must be created with New or MustNew")
	}

	current := new(state[T])
	current.done = make(chan struct{})

	p.current.Store(current)

	go p.initialize(current)

	return current
}

func (s *state[T]) result() (T, error) {
	if s.panicked {
		panic(s.panicValue)
	}

	return s.value, s.err
}

// abandon reports why this caller stopped waiting, unless initialization
// settled in the same instant the caller's context ended.
//
// The error wraps context.Cause(ctx), so a context.WithCancelCause reason
// stays reachable through errors.Is and errors.As. It describes the caller's
// own context rather than the singleton, which keeps initializing.
func (s *state[T]) abandon(ctx context.Context) (T, error) {
	if s.settled.Load() {
		return s.result()
	}

	var zero T

	return zero, fmt.Errorf(
		"singleton: waiting for initialization: %w",
		context.Cause(ctx),
	)
}

func (p *Provider[T]) initialize(current *state[T]) {
	defer func() {
		if value := recover(); value != nil {
			current.panicked = true
			current.panicValue = value
		}

		current.settled.Store(true)
		close(current.done)
	}()

	value, err := p.retrier.Do(context.Background(), p.factory)
	if err != nil {
		current.err = err

		return
	}

	current.value = value
}
