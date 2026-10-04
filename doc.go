// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

// Package singleton provides lazy, shared initialization with retries.
//
// Create a [Provider] with [New] or [MustNew]. Its first Get starts one shared
// initialization run. Failed factory attempts are retried with exponential
// backoff and jitter until the attempt budget or initialization timeout is spent.
// Use [Permanent] to stop retrying an error immediately.
//
// Initialization uses a package-owned context. Each Get waits under its caller's
// context, so canceling a caller stops only that caller's wait. Caller errors
// wrap context.Cause(ctx), preserving custom cancellation causes.
//
// A successful value, initialization failure, or factory panic is cached.
// Reset discards a failed or panicked run so the next Get can try again; it does
// nothing after success or while initialization is in progress.
//
// Shared initialization failures are *[InitError] values. Classify them with
// [InitError.Reason] and inspect the final factory error with [InitError.Err].
// Their error chain exposes both the factory error and the public stop error.
// [NewInitError] constructs a standalone failure that unwraps only its cause.
//
// This package requires Go 1.26.6 or later.
package singleton
