package application

import (
	"sync"
	"sync/atomic"

	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

// Provider lazily initializes and returns one shared value.
//
// Build one with [NewProvider]. The zero value is not usable and a Provider
// must not be copied after first use. It is safe for concurrent use.
type Provider[T any] struct {
	factory ports.Operation[T]
	retrier ports.Retrier[T]

	mu      sync.Mutex
	current atomic.Pointer[state[T]]
}

type state[T any] struct {
	settled atomic.Bool
	done    chan struct{}

	value T
	err   error

	panicked   bool
	panicValue any
}
