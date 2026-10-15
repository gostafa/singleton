// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package domain

// Safe returns an observer that recovers from panics in observer, or nil if observer is nil.
//
// Instrumentation must never become the singleton's result. Without this, a
// panicking observer is caught by the recover that guards factory panics, and
// every later call for the life of the process re-panics with it. The
// composition root applies Safe before handing an observer to any adapter, so
// no adapter can violate the invariant.
func Safe(observer RetryObserver) RetryObserver {
	if observer == nil {
		return nil
	}

	return func(event RetryEvent) {
		defer func() {
			//nolint:errcheck // Observer panic values are deliberately discarded.
			_ = recover()
		}()

		observer(event)
	}
}
