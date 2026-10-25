# Parallel and Sequential Errors

An operation can stop for several reasons: concurrent work can fail together, or cleanup can fail after the main operation. Effect 4 keeps them in one flat cause.

[Upstream page](https://effect.website/docs/v4/error-management/parallel-and-sequential-errors)

## Representing multiple failures

```vibe
// run: Console.log(JSON.stringify(failureKinds()))
function failureKinds() {
  const cause = Cause.fail("delivery rejected").combine(Cause.die(new Error("release invariant failed")))
  return cause.reasons.map((reason) => reason._tag)
}
// ["Fail", "Die"]
```

```output
["Fail","Die"]
```

A `Cause.Cause<E>` holds a readonly array of `Cause.Reason<E>`, each `Fail` with an `error`, `Die` with a `defect`, or `Interrupt` with interruption information. There is no sequential or parallel tree to traverse.

## Combining concurrent or sequential reasons

```vibe
// run: Console.log(JSON.stringify(inspectDelivery()))
function operationWithBrokenCleanup() {
  try {
    throw "delivery rejected"
  } finally {
    die(new Error("release invariant failed"))
  }
}

function inspectDelivery() {
  const exit = Effect.exit(operationWithBrokenCleanup())
  if (exit.isFailure()) {
    return {
      expected: exit.cause.hasFails(),
      defective: exit.cause.hasDies(),
      interrupted: exit.cause.hasInterrupts()
    }
  }
  return { expected: false, defective: false, interrupted: false }
}
```

```output
{"expected":false,"defective":true,"interrupted":false}
```

A cleanup failure is kept beside the original failure; when either is a defect, each expected failure becomes a defect of the same error (D305). Concurrent combinators normally stop after a failure and interrupt siblings, but overlapping failures can still add several reasons. The flat representation keeps no causal tree and doesn't prove which task completed first.

Ordinary `catch`, like `Effect.catch`, doesn't recover a cause containing a defect, whose typed failures are defects by then (D305), and recovers a typed failure beside an interruption, dropping the interruption (D306); `Effect.exit` or `Effect.catchCause` examines the whole outcome. `Cause.findError` and `Cause.findDefect` return a `Result` holding one matching value; inspect `reasons` when all values matter.

When every reason is an expected failure, ordinary catch receives the first and handles the whole cause: if both programs passed to `Effect.race` fail, catch receives one error, not an array. Use `Effect.catchCause` when recovery needs them all.

## Runtime causes and domain error collections

```vibe
// run: Console.log(JSON.stringify(Effect.flip(() => validatePorts()).map((error) => error.port)))
error InvalidPort { port: number }

function validatePorts() {
  return Effect.validate([8080, -1, 70000], (port) => {
    if (port < 1 || port > 65535) throw new InvalidPort({ port })
    return port
  })
}
// number[]![InvalidPort, ...InvalidPort[]]!
```

```output
[-1,70000]
```

Validation produces one typed failure holding a collection of domain errors, not several runtime cause reasons. Use `Effect.partition` to keep successful values alongside the collected errors.

## Coming from Effect

Effect 4's flat cause applies unchanged. Use `finally` for cleanup: its failure joins the operation's original cause, as `inspectDelivery` shows. A timeout boundary currently drops failures from the interrupted source's cleanup; see [Timing Out](019-timing-out.md).
