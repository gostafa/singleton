// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

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
	retrier ports.Retrier[T]
	factory ports.Operation[T]
	current atomic.Pointer[state[T]]
	mu      sync.Mutex
}

type state[T any] struct {
	value      T
	err        error
	panicValue any
	done       chan struct{}
	settled    atomic.Bool
	panicked   bool
}
