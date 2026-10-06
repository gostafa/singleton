// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package singleton_test

import (
	"testing"

	"github.com/gostafa/singleton"
)

const unknownReason = "initialization failed"

func TestFailureReasonString(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		want   string
		reason singleton.FailureReason
	}{
		"permanent":         {reason: singleton.FailurePermanent, want: "permanent failure"},
		"exhausted":         {reason: singleton.FailureExhausted, want: "retries exhausted"},
		"timed out":         {reason: singleton.FailureTimedOut, want: "initialization timed out"},
		"canceled":          {reason: singleton.FailureCanceled, want: "initialization canceled"},
		"the unnamed zero":  {reason: 0, want: unknownReason},
		"one past the last": {reason: singleton.FailureCanceled + 1, want: unknownReason},
		"far past the last": {reason: 200, want: unknownReason},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := test.reason.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}
