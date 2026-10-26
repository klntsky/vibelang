# Deferred

A `Deferred<A, E>` is a result another fiber can complete once. Use it to signal readiness, hand a value to waiting work, or share a one-time failure. Waiting suspends the fiber; other work keeps running.

## Creating a one-time result

```vibe
// run: const signal = makeStartupSignal(); Console.log(signal.isDone()); Console.log(signal.poll().isNone())
error StartupFailed {
  stage: string
}

function makeStartupSignal() {
  return Deferred.make<string, StartupFailed>()
}
// Deferred.Deferred<string, StartupFailed>!!
// Output: false
// Output: true
```

A new deferred is empty, holding neither a success nor a failure. Creating it is an Effect operation, so it belongs inside a program. Pass the same handle to the producer and every waiter that shares its result.

## Waiting for completion

```vibe
// run: Console.log(waitForAddress())
function waitForAddress() {
  const address = Deferred.make<string>()
  const listener = fork address.await()
  address.succeed("127.0.0.1:8080")
  return join listener
}
// Output: 127.0.0.1:8080
```

`address.await()` (`Deferred.await`) waits while the handle is empty and returns at once after completion. Several fibers can await one deferred and receive the same result. Cancelling one waiter doesn't require completing the deferred for the others.

## Completing with success or failure

```vibe
// run: Console.log(JSON.stringify(completeOnce())); Console.log(shareFailure())
function completeOnce() {
  const result = Deferred.make<number, string>()
  const accepted = result.succeed(25)
  const ignored = result.fail("too late")
  return { accepted, ignored, value: result.await() }
}
// Returns { accepted: true, ignored: false, value: 25 }.

function shareFailure() {
  const result = Deferred.make<number, string>()
  result.fail("configuration missing")
  try {
    return result.await()
  } catch (error) {
    Console.log(error)
    return 0
  }
}
// Output: {"accepted":true,"ignored":false,"value":25}
// Output: configuration missing
// Output: 0
```

The first completion wins; later completions return `false`. `Deferred.done` takes an `Exit`, `Deferred.failCause` a cause, and `Deferred.interrupt` completes waiters with interruption. `Deferred.complete(deferred, program)` evaluates a program and shares its outcome; `completeWith` stores the program for each waiter to evaluate, so it doesn't promise one execution.

## Polling completion state

```vibe
// run: Console.log(JSON.stringify(inspectWithoutWaiting()))
function inspectWithoutWaiting() {
  const result = Deferred.make<number>()
  const before = result.isDone() // false
  result.succeed(9)
  const observed = result.poll()
  if (observed.isSome()) {
    const stored = observed.value
    return { before, value: stored() }
  }
  return { before, value: undefined }
}
// Output: {"before":false,"value":9}
```

`isDone` reports whether completion happened. `poll` returns `None` while empty, else `Some` of the completed Effect, which runs when called. A completed failure is still done; calling its stored program propagates the failure.

## Coordinating work across fibers

```vibe
// run: Console.log(coordinate())
function coordinate() {
  const prepared = Deferred.make<ReadonlyArray<string>>()
  const producer = fork {
    sleep("20 millis")
    prepared.succeed(["part-a", "part-b"])
  }
  const consumer = fork {
    const parts = prepared.await()
    return parts.join("+")
  }
  const result = join consumer
  join producer
  return result
}
// Output: part-a+part-b
```

The consumer can't proceed before the producer supplies its input, whichever fiber starts first. A producer that fails before completing the signal leaves waiters waiting, so real startup code also completes it with the failure, or supervises producer failure alongside the wait. A deferred is single use; use a Queue for repeated messages or a Latch for a reusable gate.
