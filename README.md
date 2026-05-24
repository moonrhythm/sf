# sf

[![Go Reference](https://pkg.go.dev/badge/github.com/moonrhythm/sf.svg)](https://pkg.go.dev/github.com/moonrhythm/sf)

`sf` is a generic, context-aware [singleflight](https://pkg.go.dev/golang.org/x/sync/singleflight)
helper for Go. It collapses concurrent calls that share the same key into a
single execution and returns the result to every caller, while adding three
things the standard `singleflight` does not:

- **Generics** — `Do[T]` returns a typed `T`, not `any`.
- **Smart context handling** — a single caller cancelling its context returns
  *that* caller early without killing the shared work that others are still
  waiting on. The shared work is cancelled only once *every* waiter has gone away.
- **A global kill switch** — `SetDisable` lets you bypass deduplication entirely.

## Install

```sh
go get github.com/moonrhythm/sf
```

Requires Go 1.25+.

## Usage

```go
import "github.com/moonrhythm/sf"

func getUser(ctx context.Context, id string) (*User, error) {
	user, err, shared := sf.Do(ctx, "user:"+id, func(ctx context.Context) (*User, error) {
		return db.LoadUser(ctx, id)
	})
	if err != nil {
		return nil, err
	}
	_ = shared // true if this result came from another in-flight call
	return user, nil
}
```

When many goroutines call `getUser` for the same `id` at the same time, the
expensive `db.LoadUser` runs only once and all callers receive the same result.

### No return value

Use `DoVoid` for work that only reports an error:

```go
err, shared := sf.DoVoid(ctx, "refresh:cache", func(ctx context.Context) error {
	return cache.Refresh(ctx)
})
```

### Forget a key

Drop the cached in-flight call so the next `Do` re-runs the function:

```go
sf.Forget("user:" + id)
```

### Disable globally

When disabled, every `Do`/`DoVoid` call runs its function directly with no
deduplication (and `shared` is always `false`). Useful in tests or for
temporarily turning the behavior off.

```go
sf.SetDisable(true)
```

## Behavior

### Context

The function you pass to `Do` receives a **new** context derived from your
context via `context.WithoutCancel`. This means:

- Values from the calling context are preserved.
- The function is **not** cancelled when an individual caller's context is
  cancelled — other waiters still depend on its result.
- Once the last waiter leaves (its context is cancelled, or it receives the
  result), the shared context is cancelled. If the function is still running and
  nobody is left to consume its result, it gets cancelled too.

### Per-caller cancellation

If a caller's context is cancelled before the shared function finishes, that
caller returns immediately with `ctx.Err()` (e.g. `context.Canceled`) and a
`shared` value of `false`. The shared function keeps running for the remaining
waiters.

### Return values

```go
func Do[T any](ctx context.Context, key string, fn func(context.Context) (T, error)) (T, error, bool)
```

| Return  | Meaning                                                                 |
| ------- | ----------------------------------------------------------------------- |
| `T`     | The result of `fn` (zero value of `T` on error).                        |
| `error` | The error from `fn`, or the caller's `ctx.Err()` if it was cancelled.   |
| `bool`  | `shared` — whether this result was shared with other in-flight callers. |

## API

| Function                                | Description                                                              |
| --------------------------------------- | ------------------------------------------------------------------------ |
| `Do[T](ctx, key, fn) (T, error, bool)`  | Deduplicate calls by `key` and return a typed result.                    |
| `DoVoid(ctx, key, fn) (error, bool)`    | Same as `Do` for functions that only return an `error`.                  |
| `Forget(key)`                           | Remove the in-flight call for `key` so the next call re-runs `fn`.       |
| `SetDisable(value bool)`                | Globally enable/disable deduplication. Safe to call from any goroutine.  |

## Note on keys

As with the standard library, keys are global to the process and deduplication
only spans calls that overlap in time. Reusing one key for functions with
different return types `T` is unsafe: a caller can receive a value produced by
another caller's still-in-flight function, causing a type assertion panic. Use
distinct keys (or distinct types) per logical operation.

## License

[MIT](LICENSE)
