# Deferred

A `Deferred<A, E>` is a result that another fiber can complete once. Use it to signal readiness, hand a value to waiting work, or share a one-time failure. Waiting suspends the fiber while allowing other work to run.

## Creating a one-time result

```ts
import { Deferred } from "effect"

class StartupFailed extends TaggedError("StartupFailed")<{
  readonly stage: string
}> {}

function makeStartupSignal() {
  return Deferred.make<string, StartupFailed>()
}
// Effect<Deferred.Deferred<string, StartupFailed>, never, never>
```

The new deferred starts empty and contains neither a success nor a failure. Creating it is an Effect operation and belongs inside a program. Pass the same handle to the producer and every waiter that should share its result.

## Waiting for completion

```ts
import { Deferred, Fiber } from "effect"

function waitForAddress() {
  const address = Deferred.make<string>()
  const listener = fork(() => Deferred.await(address))
  Deferred.succeed(address, "127.0.0.1:8080")
  return Fiber.join(listener)
}
```

`Deferred.await` waits while the handle is empty and returns immediately after completion. Several fibers can await the same deferred and receive the same result. Cancelling one waiter does not require completing the deferred for everyone else.

## Completing with success or failure

```ts
import { Deferred } from "effect"

function completeOnce() {
  const result = Deferred.make<number, string>()
  const accepted = Deferred.succeed(result, 25)
  const ignored = Deferred.fail(result, "too late")
  return { accepted, ignored, value: Deferred.await(result) }
}
// Returns { accepted: true, ignored: false, value: 25 }.

function shareFailure() {
  const result = Deferred.make<number, string>()
  Deferred.fail(result, "configuration missing")
  try {
    return Deferred.await(result)
  } catch (error) {
    Effect.logError(error)
    return 0
  }
}
```

The first completion wins, and completion operations return `false` when a result already exists. `Deferred.done` accepts an `Exit`, `Deferred.failCause` accepts a cause, and `Deferred.interrupt` completes waiters with interruption. `Deferred.complete(deferred, program)` evaluates a program and shares its outcome; `completeWith` stores the program for each waiter to evaluate, so it does not promise one execution of that program.

## Polling completion state

```ts
import { Deferred, Option } from "effect"

function inspectWithoutWaiting() {
  const result = Deferred.make<number>()
  const before = Deferred.isDone(result) // false
  Deferred.succeed(result, 9)
  const observed = Deferred.poll(result)
  if (Option.isSome(observed)) {
    const stored = observed.value
    return { before, value: stored() }
  }
  return { before, value: undefined }
}
```

`isDone` reports whether completion has happened. `poll` returns `None` while empty or `Some` containing the completed Effect, which runs when called. A completed failure is still done, and calling its stored program propagates that failure.

## Coordinating work across fibers

```ts
import { Deferred, Fiber } from "effect"

function coordinate() {
  const prepared = Deferred.make<ReadonlyArray<string>>()
  const producer = fork(() => {
    Effect.sleep("20 millis")
    Deferred.succeed(prepared, ["part-a", "part-b"])
  })
  const consumer = fork(() => {
    const parts = Deferred.await(prepared)
    return parts.join("+")
  })
  const result = Fiber.join(consumer)
  Fiber.join(producer)
  return result
}
```

The consumer cannot proceed before the producer supplies its input, regardless of which fiber starts first. A producer that fails before completing the signal leaves waiters waiting, so real startup code should complete it with failure too, or supervise producer failure alongside the wait. A deferred is single use; use a Queue for repeated messages or a Latch for a reusable gate.
