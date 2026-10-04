# singleton

Lazy, retryable, process-local singletons for Go.

One shared value, initialized once, on first use — with exponential backoff, a bounded retry budget, and one property most `sync.Once`-based helpers get wrong: **a caller that gives up cannot poison the singleton for the rest of the process.**

```go
var redis = singleton.MustNew(func(ctx context.Context) (*redis.Client, error) {
    return dialRedis(ctx)
})

func Handler(w http.ResponseWriter, r *http.Request) {
    client, err := redis.Get(r.Context())   // request ctx bounds *this* wait only
    ...
}
```

---

## Contents

- [Why](#why)
- [Install](#install)
- [Quick start](#quick-start)
- [How it behaves](#how-it-behaves)
- [Examples](#examples)
- [API reference](#api-reference)
- [Defaults](#defaults)
- [Gotchas](#gotchas)
- [Architecture](#architecture)

---

## Why

The usual lazy singleton is `sync.OnceValue`. It has two failure modes that matter in a server:

**1. A cancelled request poisons the singleton.** If you pass the request context into the initializer, the first request to time out permanently caches `context.DeadlineExceeded` as the singleton's value. Every later request gets that error forever.

**2. There is no retry.** A dependency that is down for two seconds at startup fails initialization once, and the failure is cached for the process lifetime.

This package separates the two clocks. Initialization runs on its own goroutine under a **package-owned context**; `Get` waits on the result under the **caller's** context. Cancelling a caller stops that caller waiting — nothing else.

```
caller ctx  ──cancel──►  Get returns context.Canceled
                          (initialization keeps running)
init ctx    ──────────►  factory, retried, bounded by WithInitializationTimeout
```

## Install

```sh
go get github.com/mostafakhairy0305-dot/singleton
```

```go
import "github.com/mostafakhairy0305-dot/singleton"
```

**Requires Go 1.26.6+**. One dependency: [`cenkalti/backoff/v7`](https://github.com/cenkalti/backoff), fully quarantined behind an internal adapter — none of its types appear in this package's API.

## Quick start

```go
package cache

import (
    "context"
    "fmt"
    "time"

    "github.com/mostafakhairy0305-dot/singleton"
    "github.com/redis/go-redis/v9"
)

var client = singleton.MustNew(func(ctx context.Context) (*redis.Client, error) {
    c := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

    if err := c.Ping(ctx).Err(); err != nil {
        // Clean up before returning an error: this attempt's value is discarded.
        _ = c.Close()

        return nil, fmt.Errorf("ping redis: %w", err)
    }

    return c, nil
}, singleton.WithMaxAttempts(5), singleton.WithInitializationTimeout(30*time.Second))

// Client returns the shared Redis client, dialing it on first call.
func Client(ctx context.Context) (*redis.Client, error) {
    return client.Get(ctx)
}
```

Nothing runs until the first `Get`. Fifty concurrent `Get` calls produce exactly one dial.

## How it behaves

### Lazy and single-flight

The factory runs on the first `Get` and exactly once, no matter how many goroutines call concurrently. Every caller receives the same value.

### Caller cancellation is isolated

```go
ctx, cancel := context.WithCancel(context.Background())
cancel()

_, err := p.Get(ctx)          // wraps context.Canceled — this caller gave up
                              // initialization is still running

value, err := p.Get(context.Background())   // still gets the value
```

`Get` wraps `context.Cause(ctx)`, so a `context.WithCancelCause` reason reaches the caller through `errors.Is` and `errors.As`. If initialization completes at the same instant the caller's context dies, the completed result wins.

### Retry, then give up

Failures are retried with exponential backoff and jitter until either the attempt budget or the initialization timeout is exhausted. Return [`Permanent(err)`](#func-permanent) to stop immediately — no point retrying a malformed config.

### Failure is cached until you `Reset`

Once initialization fails, every `Get` returns that same error. This is deliberate: without it, each arriving request would trigger a fresh retry storm against a dependency that is already struggling.

Call [`Reset`](#func-providerreset) to discard a failed initialization so the next `Get` starts fresh — typically from a health check or a supervisor loop.

```go
if _, err := pool.Get(ctx); err != nil {
    pool.Reset()   // next Get will try again
}
```

`Reset` is a **no-op after success** (a live value other goroutines already hold is never torn down — there is no close hook) and a **no-op while initialization is in flight** (callers already waiting are never stranded).

### Panics

A panic in the factory is recorded and **re-panics in every caller**, rather than silently handing back a zero value — the same contract as `sync.OnceValue`. `Reset` clears a panicked initialization.

A panic in a retry observer is recovered and discarded. Instrumentation must never become the singleton's result.

## Examples

### Permanent failures

Retrying cannot fix bad configuration, so don't burn the budget on it:

```go
var db = singleton.MustNew(func(ctx context.Context) (*pgxpool.Pool, error) {
    cfg, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
    if err != nil {
        // Stops after this attempt. Reason will be FailurePermanent.
        return nil, singleton.Permanent(fmt.Errorf("parse database url: %w", err))
    }

    pool, err := pgxpool.NewWithConfig(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("connect: %w", err)   // retried
    }

    if err := pool.Ping(ctx); err != nil {
        pool.Close()

        return nil, fmt.Errorf("ping: %w", err)      // retried
    }

    return pool, nil
})
```

`errors.Is` still sees through `Permanent`, so wrapping loses nothing.

### Classifying failures

Branch on `Reason()` — **not** on `errors.Is`. See [Gotchas](#classify-with-reason-not-errorsis).

```go
client, err := provider.Get(ctx)
if err != nil {
    var initErr *singleton.InitError
    if errors.As(err, &initErr) {
        switch initErr.Reason() {
        case singleton.FailurePermanent:
            log.Error("misconfigured, not retrying", "err", initErr.Err())
        case singleton.FailureExhausted:
            log.Warn("dependency down, retry budget spent", "err", initErr.Err())
        case singleton.FailureTimedOut:
            log.Warn("initialization exceeded its deadline", "err", initErr.Err())
        }

        return err
    }

    // Not an InitError: this caller's own context ended.
    return err   // context.Canceled / context.DeadlineExceeded
}
```

Distinguishing "the singleton failed" from "my request was cancelled" is exactly the `errors.As` check above: initialization failures are always `*InitError`, caller-context failures never are.

### Observability

```go
provider := singleton.MustNew(dialRedis,
    singleton.WithRetryObserver(func(e singleton.RetryEvent) {
        metrics.InitRetries.Inc()
        log.Warn("redis init retry",
            "attempt", e.Attempt,
            "err", e.Err,
            "retry_in", e.NextDelay,
        )
    }),
)
```

One event per *retried* attempt — a run of 3 attempts ending in success emits 2 events, with `Attempt` 1 then 2. The final attempt emits nothing; its outcome is the return value of `Get`.

### Testing against the interface

`Interface[T]` lets consumers substitute a fake without touching this package:

```go
type Service struct {
    cache singleton.Interface[*redis.Client]
}

// In tests:
type fakeProvider struct{ client *redis.Client }

func (f fakeProvider) Get(context.Context) (*redis.Client, error) { return f.client, nil }
func (f fakeProvider) Reset()                                     {}

svc := &Service{cache: fakeProvider{client: miniredisClient}}
```

### Recovering from a startup outage

```go
func (h *Health) Check(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    if _, err := h.db.Get(ctx); err != nil {
        var initErr *singleton.InitError
        if errors.As(err, &initErr) && initErr.Reason() != singleton.FailurePermanent {
            h.db.Reset()   // transient — let the next check re-dial
        }

        return err
    }

    return nil
}
```

### Handling construction errors

`MustNew` panics on invalid options, which is what you want at package scope. Use `New` when options are computed at runtime:

```go
provider, err := singleton.New(factory,
    singleton.WithMaxAttempts(cfg.Attempts),          // errors if 0
    singleton.WithRetryInterval(cfg.Base, cfg.Max),   // errors if Max < Base
)
if err != nil {
    return fmt.Errorf("configure pool: %w", err)
}
```

Neither reports *factory* failures — those surface at `Get`.

---

## API reference

Everything below is the complete exported surface. All other packages are under `internal/` and cannot be imported.

### Constructors

#### `func New`

```go
func New[T any](factory Factory[T], options ...Option) (*Provider[T], error)
```

Creates a provider. Does not call the factory. Returns an error if `factory` is nil, if any option is invalid, or if a zero-value `Option{}` is passed.

#### `func MustNew`

```go
func MustNew[T any](factory Factory[T], options ...Option) *Provider[T]
```

Same as `New`, panicking instead of returning an error. Intended for package-level variables. **Panics only for invalid construction options, never for factory failures.**

#### `func Permanent`

```go
func Permanent(err error) error
```

Marks a factory error as non-retriable. Initialization stops at that attempt with `Reason() == FailurePermanent`. `Permanent(nil)` returns `nil`. The wrapped error stays reachable through `errors.Is` / `errors.As`.

The concrete result is a `*PermanentError`, which callers can inspect with `errors.As`. A factory may also return a `*PermanentError` directly, including wrapped in another error.

### Error values

Use `errors.Is` to match construction errors returned by `New` or panicked by `MustNew`:

| Error | Meaning |
| --- | --- |
| `ErrNilFactory` | The factory is nil |
| `ErrInvalidOption` | A zero-value `Option{}` was passed |
| `ErrZeroMaxAttempts` | The attempt budget is zero |
| `ErrNegativeTimeout` | The initialization timeout is negative |
| `ErrZeroInitialInterval` | The initial retry interval is not positive |
| `ErrMaxIntervalBelowInitial` | The maximum retry interval is less than the initial interval |

`InitError.Unwrap` exposes the factory error and a public stop error: `ErrPermanent`, `ErrRetriesExhausted`, `context.DeadlineExceeded`, or `context.Canceled`. Retry-library stop errors stay internal; callers that previously matched them should use these public equivalents. Classify initialization with `InitError.Reason()` because a factory error can also match a stop error.

### `type Provider`

```go
type Provider[T any] struct { /* unexported */ }
```

The lazy singleton. Create with `New` or `MustNew`; the zero value is unusable and must not be copied after first use.

#### `func (*Provider[T]) Get`

```go
func (p *Provider[T]) Get(ctx context.Context) (T, error)
```

Returns the shared value, starting initialization on first call and blocking until it settles.

| Outcome | Returns |
| --- | --- |
| Success | the shared value, `nil` |
| Initialization failed | the **zero** `T`, an `*InitError` |
| Caller's context ended first | the **zero** `T`, an error wrapping `context.Cause(ctx)` |
| Factory panicked | re-panics with the factory's panic value |

Panics if `ctx` is nil, or if called on a zero-value `Provider`.

#### `func (*Provider[T]) Reset`

```go
func (p *Provider[T]) Reset()
```

Discards a **failed or panicked** initialization so the next `Get` starts a new one. No-op after success, no-op while initialization is in flight, no-op before the first `Get`. Safe to call concurrently.

### `type Interface`

```go
type Interface[T any] interface {
    Get(ctx context.Context) (T, error)
    Reset()
}
```

The contract `*Provider[T]` satisfies. Depend on this in consumers that need to substitute a fake.

### `type Factory`

```go
type Factory[T any] = func(context.Context) (T, error)
```

Creates the singleton value. The context is **owned by this package** — it carries the initialization timeout, not any caller's deadline.

> **A factory that returns an error must release whatever it already acquired.** Every failed attempt's return value is discarded, so a factory that dials a connection and *then* fails validation leaks one connection per attempt.

### `type InitError`

```go
type InitError struct { /* unexported */ }

func NewInitError(reason FailureReason, err error) *InitError
func (e *InitError) Reason() FailureReason
func (e *InitError) Err() error

func (e *InitError) Error() string
func (e *InitError) Unwrap() []error
```

Returned whenever shared initialization fails — always, so `errors.As(err, &initErr)` has no silent false branch.

`Error()` formats as `singleton: <reason>: <err>`, e.g. `singleton: initialization timed out: dial: boom`.

`Unwrap` returns both the factory error and the retry loop's public stop error, so `errors.Is` matches either:

```go
errors.Is(err, myFactoryError)               // true
errors.Is(err, context.DeadlineExceeded)     // true, for a timeout
errors.Is(err, singleton.ErrRetriesExhausted) // true, for an exhausted budget
errors.Unwrap(err)                           // nil — this is a multi-error Unwrap
```

Repeated `Get` calls return the same cached `*InitError` until `Reset`. `NewInitError(reason, err)` constructs a standalone failure that unwraps only `err`. The zero value unwraps no errors.

Migration: replace `initErr.Reason` and `initErr.Err` field reads with `initErr.Reason()` and `initErr.Err()`. Replace `&InitError{Reason: reason, Err: err}` with `NewInitError(reason, err)`. The fields are now private so callers cannot overwrite a cached failure shared with other goroutines.

### `type PermanentError`

```go
type PermanentError struct {
    Err error
}

func (e *PermanentError) Error() string
func (e *PermanentError) Unwrap() error
```

Marks the wrapped error as non-retriable. `Error` returns the wrapped error's message, and `Unwrap` returns `Err`. Create one with `Permanent(err)` or return a `*PermanentError` directly from the factory.

### `type FailureReason`

```go
type FailureReason uint8
func (r FailureReason) String() string
```

| Constant | `String()` | Meaning |
| --- | --- | --- |
| `FailurePermanent` | `permanent failure` | Factory returned `Permanent(err)` |
| `FailureExhausted` | `retries exhausted` | Attempt budget spent (`WithMaxAttempts`) |
| `FailureTimedOut` | `initialization timed out` | `WithInitializationTimeout` elapsed |
| `FailureCanceled` | `initialization canceled` | Reserved — see [Gotchas](#failurecanceled-is-currently-unreachable) |

### `type RetryEvent`

```go
type RetryEvent struct {
    Attempt   uint           // 1-based number of the attempt that just failed
    Err       error          // why it failed
    NextDelay time.Duration  // wait before the next attempt
}
```

### `type Option`

```go
type Option struct { /* unexported */ }
```

Opaque by design, so callers cannot depend on the retry library underneath. A zero-value `Option{}` is rejected by `New`.

| Option | Signature | Validation |
| --- | --- | --- |
| `WithMaxAttempts` | `(n uint) Option` | errors if `n == 0` |
| `WithInitializationTimeout` | `(timeout time.Duration) Option` | errors if negative; **`0` disables the timeout** |
| `WithRetryInterval` | `(initial, maximum time.Duration) Option` | errors if `initial <= 0` or `maximum < initial` |
| `WithRetryObserver` | `(observer func(RetryEvent)) Option` | `nil` accepted (disables) |

`WithMaxAttempts` counts **total attempts including the first** — `WithMaxAttempts(1)` never retries.

## Defaults

| Setting | Default |
| --- | --- |
| Max attempts | `5` |
| Initialization timeout | `30s` |
| Initial retry interval | `250ms` |
| Max retry interval | `5s` |
| Backoff multiplier | `2` (not configurable) |
| Jitter | ±20% (not configurable) |

With the defaults, 5 attempts are separated by 4 delays with nominal values `250ms → 500ms → 1s → 2s`. Jitter shifts each by up to ±20%, so a fully exhausted retry budget takes roughly **3–4.5s** — well inside the 30s timeout. A measured run:

```
attempts = 5
delays   = [213ms 409ms 1.169s 1.653s]
elapsed  = 3.45s
reason   = retries exhausted
```

## Gotchas

### Classify with `Reason`, not `errors.Is`

`Unwrap` deliberately exposes both the factory error and the stop reason, so `errors.Is` answers *"does this appear anywhere in the chain"* — a different question from *"why did initialization stop"*.

A factory doing network I/O under a per-attempt deadline returns `context.DeadlineExceeded` every time. After the budget is spent:

```go
initErr.Reason()                            // FailureExhausted — correct
errors.Is(err, context.DeadlineExceeded)  // ALSO true — the factory's error
```

`Reason()` is the authoritative classification. Reserve `errors.Is` for matching your own sentinel errors.

### The factory owns its cleanup

Restated because it leaks sockets quietly: every failed attempt's return value is dropped. Close what you opened before returning an error.

### A failed singleton stays failed

By design. Nothing re-initializes automatically — wire `Reset` into a health check if you want recovery.

### `Get` panics on misuse

Nil context and zero-value `Provider` panic rather than returning errors, matching stdlib precedent (`context.WithCancel`). Both indicate a programming error, not a runtime condition.

### `FailureCanceled` is currently unreachable

The initialization context descends from `context.Background()` with only a timeout, and nothing cancels it — so no failure is classified `FailureCanceled` today. The constant exists for a future shutdown hook. Don't write a code path that depends on receiving it.

### Inspecting permanent errors

`Permanent(err)` returns a public `*PermanentError`. Inspect the marker with `errors.As`, match its wrapped error with `errors.Is`, or read `InitError.Reason()` after initialization stops. The final initialization error preserves the marker's wrapped error in `InitError.Err()`.

## Architecture

Ports and adapters, with everything but the facade under `internal/`:

```
types.go                           public types and private boundary state
funcs.go                           facade, options, error accessors and conversions
consts.go, vars.go                  defaults, failure reasons and error values
internal/
  domain/                          failure contracts and retry event payloads
  ports/                           the outbound Retrier port
  application/                     lazy single-flight state machine
  adapters/backoffretry/           retry engine and concrete internal failures
```

Public types are defined in `singleton` and reference only public or standard-library types in their exported fields and signatures. Private adapters translate permanent markers, retry events, and initialization failures between this API and the internal implementation. Failures are converted before caching so callers share one public error instance.

The `Retrier` port is what keeps `cenkalti/backoff` out of the API entirely, and it carries three obligations any adapter must honour: run the operation at least once; return the **zero** `T` on failure, never a half-built value; and set `Reason` from the policy's own stop condition rather than inferring it from the operation's error.

`go list -deps` on `internal/application`, `internal/domain` and `internal/ports` resolves **zero** backoff imports — the core does not know a retry engine exists.
