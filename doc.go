// Package singleton provides lazy, retryable, process-local singletons.
//
// A [Provider] initializes one shared value on first use and returns that same
// value to every later caller. Initialization runs on its own goroutine under a
// context owned by this package, while Get waits under the caller's context.
// Cancelling a caller therefore stops only that caller waiting: a request that
// times out cannot poison the singleton for the rest of the process.
//
// Failed attempts are retried with exponential backoff and jitter until the
// attempt budget or the initialization timeout is spent, after which the
// failure is cached. Wrap an error with [Permanent] to stop retrying at once,
// and call Reset to discard a failed initialization so the next Get starts a
// new one.
//
//	var client = singleton.MustNew(func(ctx context.Context) (*redis.Client, error) {
//		c := redis.NewClient(options)
//
//		if err := c.Ping(ctx).Err(); err != nil {
//			_ = c.Close()
//
//			return nil, fmt.Errorf("ping redis: %w", err)
//		}
//
//		return c, nil
//	})
//
//	func Client(ctx context.Context) (*redis.Client, error) {
//		return client.Get(ctx)
//	}
//
// This package requires Go 1.24 or later.
package singleton
