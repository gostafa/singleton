package backoffretry

import (
	"time"

	"github.com/mostafakhairy0305-dot/singleton/internal/domain"
)

// Config tunes the retry policy.
type Config struct {
	// MaxAttempts is the total number of attempts, including the first.
	MaxAttempts uint

	// Timeout bounds the whole retry loop. Zero disables it.
	Timeout time.Duration

	// InitialInterval is the first delay between attempts.
	InitialInterval time.Duration

	// MaxInterval is the ceiling the delay grows toward.
	MaxInterval time.Duration

	// Observer receives one event per retried attempt. It must already be
	// panic-safe; see [domain.RetryObserver.Safe].
	Observer domain.RetryObserver
}

// Retrier runs an operation with exponential backoff and jitter.
//
// It implements [ports.Retrier].
type Retrier[T any] struct {
	cfg Config
}
