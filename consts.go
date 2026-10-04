// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton

import (
	"time"
)

const (
	// FailurePermanent means the factory returned an error wrapped with
	// Permanent, so no further attempts were made.
	FailurePermanent FailureReason = iota + 1

	// FailureExhausted means the attempt budget set by WithMaxAttempts ran out.
	FailureExhausted

	// FailureTimedOut means the deadline set by WithInitializationTimeout elapsed.
	FailureTimedOut

	// FailureCanceled means the initialization context was canceled.
	// No current code path produces it; it is reserved for a future shutdown hook.
	FailureCanceled

	zeroAttempts           = 0
	defaultMaxAttempts     = 5
	defaultTimeout         = 30 * time.Second
	defaultInitialInterval = 250 * time.Millisecond
	defaultMaxInterval     = 5 * time.Second
)
