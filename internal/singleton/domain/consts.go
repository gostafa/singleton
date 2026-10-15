// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

const (
	// FailurePermanent means the factory returned an error wrapped with
	// [PermanentError], so no further attempts were made.
	FailurePermanent FailureReason = iota + 1

	// FailureExhausted means the attempt budget ran out.
	FailureExhausted

	// FailureTimedOut means the initialization deadline elapsed.
	FailureTimedOut

	// FailureCanceled means the initialization context was canceled.
	FailureCanceled
)
