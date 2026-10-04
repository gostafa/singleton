// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package ports

import (
	"context"
)

type (
	// Operation is one initialization attempt.
	//
	// The context belongs to the [Retrier], not to any caller waiting on the
	// result.
	Operation[T any] = func(context.Context) (T, error)

	// Retrier runs an operation until it succeeds or its policy gives up.
	//
	// Implementations must honor three rules, each of which exists because
	// violating it caused a real defect:
	//
	//   - Run the operation at least once.
	//
	//   - On failure return the zero T, never the last attempt's value. Handing
	//     back a partially built value alongside an error invites callers who check
	//     the error loosely to use a half-open connection.
	//
	//   - On failure return a classified initialization error whose Reason comes from the
	//     policy's own stop condition, never inferred from the operation's error.
	//     A policy that classifies by inspecting the operation error reports a
	//     timeout for an operation that returned context.DeadlineExceeded on every
	//     attempt, when what actually happened is that the retry budget ran out.
	//     Retry adapters implement domain.InitError; the public boundary converts it
	//     before the application caches it.
	Retrier[T any] interface {
		// Do runs op until it succeeds or the policy stops.
		//
		// Do owns the operation's context lifetime and derives any deadline from
		// its own policy, so ctx is only the parent.
		Do(ctx context.Context, op Operation[T]) (T, error)
	}
)
