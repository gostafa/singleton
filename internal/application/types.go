// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package application

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

type (
	// Provider lazily initializes and returns one shared value.
	//
	// Build one with [NewProvider]. The zero value is not usable and a Provider
	// must not be copied after first use. It is safe for concurrent use.
	Provider[T any] struct {
		lifecycle *lifecycle[T]
	}

	// Interface is the concurrent initialization behavior exposed to callers.
	Interface[T any] interface {
		// Get waits for the shared initialization.
		Get(ctx context.Context) (T, error)
		// Reset discards an unsuccessful initialization.
		Reset()
	}

	lifecycle[T any] = struct {
		retrier ports.Retrier[T]
		factory ports.Operation[T]
		current atomic.Pointer[state[T]]
		mu      sync.Mutex
	}

	state[T any] = struct {
		value      T
		err        error
		panicValue any
		done       chan struct{}
		settled    atomic.Bool
		panicked   bool
	}
)
