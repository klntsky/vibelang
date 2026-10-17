# Parallel and Sequential Errors

An operation can stop for several reasons: concurrent work can fail together, or cleanup can fail after the main operation. Effect 4 retains those reasons in one flat cause.

[Upstream page](https://effect.website/docs/v4/error-management/parallel-and-sequential-errors)

## Representing multiple failures

```ts
function failureKinds() {
  const cause = Cause.combine(
    Cause.fail("delivery rejected"),
    Cause.die(new Error("release invariant failed"))
  )
  return cause.reasons.map((reason) => reason._tag)
}
// ["Fail", "Die"]
```

A `Cause.Cause<E>` contains a readonly array of `Cause.Reason<E>`. Each reason is `Fail` with an `error`, `Die` with a `defect`, or `Interrupt` with interruption information. There is no separate sequential or parallel tree to traverse.

## Combining concurrent or sequential reasons

```ts
function operationWithBrokenCleanup() {
  try {
    throw "delivery rejected"
  } finally {
    Effect.die(new Error("release invariant failed"))
  }
}

function inspectDelivery() {
  const exit = Effect.exit(operationWithBrokenCleanup())
  if (Exit.isFailure(exit)) {
    return {
      expected: Cause.hasFails(exit.cause),
      defective: Cause.hasDies(exit.cause),
      interrupted: Cause.hasInterrupts(exit.cause)
    }
  }
  return { expected: false, defective: false, interrupted: false }
}
```

Cleanup failure is retained alongside the original failure. Concurrent combinators normally stop after a failure and interrupt siblings, but overlapping failures can still contribute several reasons. Their flat representation does not preserve a causal tree or prove which task completed first.

Ordinary `catch` does not recover a mixed cause containing a defect or interruption, even when a typed failure is present. Use `Effect.exit` or `Effect.catchCause` to examine the whole outcome. `Cause.findError` and `Cause.findDefect` return a `Result` for extracting one matching value; inspect `reasons` when all values matter.

When every reason is an expected failure, ordinary catch receives the first failure and handles the cause as a whole. For example, if both programs passed to `Effect.race` fail, catch receives one error, not an array. Use `Effect.catchCause` when recovery needs all the failures.

## Runtime causes and domain error collections

```ts
error InvalidPort { port: number }

function validatePorts() {
  return Effect.validate([8080, -1, 70000], (port) => {
    if (port < 1 || port > 65535) throw new InvalidPort({ port })
    return port
  })
}
// Effect<number[], [InvalidPort, ...InvalidPort[]], never>
```

Validation produces one typed failure containing a collection of domain errors. That is different from several runtime cause reasons. Use `Effect.partition` instead when you want successful values alongside the collected errors.

## Coming from Effect

Effect 4's flat cause representation applies unchanged. Use `finally` for cleanup, with Effeck's guarantee that cleanup failure never replaces and drops an earlier failure.
