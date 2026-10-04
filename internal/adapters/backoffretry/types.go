// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"time"

	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
)

// Config tunes the retry policy.
type Config struct {
	Observer        domain.RetryObserver
	MaxAttempts     uint
	Timeout         time.Duration
	InitialInterval time.Duration
	MaxInterval     time.Duration
}

// Retrier runs an operation with exponential backoff and jitter.
//
// It implements [ports.Retrier].
type Retrier[T any] struct {
	cfg Config
}
