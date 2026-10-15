// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package application coordinates lazy, shared singleton initialization.
package application

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/gostafa/singleton/internal/singleton/ports"
)

type (
	// provider lazily initializes and returns one shared value.
	//
	// Build one with [NewProvider]. The zero value is not usable and a provider
	// must not be copied after first use. It is safe for concurrent use.
	provider[T any] struct {
		lifecycle *lifecycle[T]
	}

	// Interface is the concurrent initialization behavior exposed to callers.
	Interface[T any] interface {
		// Get waits for the shared initialization.
		Get(ctx context.Context) ports.Result[T]
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
		outcome    ports.Result[T]
		panicValue any
		done       chan struct{}
		settled    atomic.Bool
		panicked   bool
	}
)
