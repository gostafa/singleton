package domain

const (
	// FailurePermanent means the factory returned an error wrapped with
	// [Permanent], so no further attempts were made.
	FailurePermanent FailureReason = iota + 1

	// FailureExhausted means the attempt budget ran out.
	FailureExhausted

	// FailureTimedOut means the initialization deadline elapsed.
	FailureTimedOut

	// FailureCanceled means the initialization context was cancelled.
	FailureCanceled
)

// chainCapacity is how many errors an [InitError] can unwrap to: the factory
// error and the retry policy's stop condition.
const chainCapacity = 2
