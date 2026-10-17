# Basic Concurrency

Ordinary Effeck calls run in sequence. Use Effect's concurrency combinators to overlap work, choose a concurrency limit, and control cancellation. Pass functions describing the work so the combinator starts each operation at the right time.

## Sequential, bounded, and unbounded work

```ts
function task(id: number) {
  Effect.logInfo("starting", id)
  Effect.sleep("40 millis")
  return id * 2
}

function runTasks() {
  const sequential = Effect.all([task(1), task(2)])
  const bounded = Effect.forEach([1, 2, 3, 4], (id) => task(id), {
    concurrency: 2
  })
  const unbounded = Effect.all([
    task(5),
    task(6),
    task(7)
  ], { concurrency: "unbounded" })
  return { sequential, bounded, unbounded }
}
```

The default is sequential execution. A positive numeric limit bounds active operations, while `"unbounded"` permits all supplied operations to run concurrently. Results retain input order even when work finishes out of order; choose a finite limit when work uses a constrained service or resource.

## Interrupting computations

```ts
function cancelWork() {
  const worker = fork(() => {
    Effect.sleep("1 minute")
    return "finished"
  })
  Fiber.interrupt(worker)
  return Fiber.await(worker)
}
```

`Fiber.interrupt` requests cancellation and waits for termination, including finalizers. `Fiber.await` then returns an `Exit` describing the interruption rather than propagating it. `Effect.interrupt()` interrupts the current fiber; interruption is outside the expected error channel and ordinary `catch` does not handle it.

## Attaching interruption cleanup

```ts
function cancelWithCleanup() {
  const worker = fork(Effect.onInterrupt(() => {
    try {
      Effect.sleep("1 minute")
    } finally {
      Effect.logInfo("released worker resources")
    }
  }, () => Effect.logInfo("worker cancelled")))
  Effect.sleep("10 millis")
  Fiber.interrupt(worker)
}
```

`finally` runs on every exit, while `onInterrupt` adds work specifically for cancellation. Cleanup runs before the interrupting caller resumes. For acquired resources, use `using` so release is registered as part of acquisition rather than depending on a later statement.

## Propagating cancellation across concurrent work

```ts
function cancelBatch() {
  Effect.forEach([1, 2, 3], (id) => {
    try {
      if (id === 2) {
        Effect.sleep("10 millis")
        Effect.interrupt()
      }
      Effect.sleep("1 second")
      return id
    } finally {
      Effect.logInfo("branch finished", id)
    }
  }, { concurrency: 3 })
}
```

An interrupted branch stops the concurrent operation and interrupts its active siblings. An expected failure also stops the default concurrent traversal and cancels work whose result is no longer needed. The parent waits for branch cleanup; a cause containing interruption or a defect passes through ordinary `catch` intact.

## Racing under different winner rules

```ts
function answer(label: string, delay: Duration.Input) {
  Effect.sleep(delay)
  return label
}

function raceExamples() {
  const firstSuccess = Effect.race(
    () => { throw "unavailable" },
    answer("replica", "20 millis")
  )
  const anySuccess = Effect.raceAll([
    () => answer("west", "40 millis"),
    () => answer("east", "10 millis"),
    () => answer("central", "30 millis")
  ])
  const firstCompletion = Effect.raceFirst(
    answer("primary", "10 millis"),
    answer("backup", "50 millis")
  )
  return { firstSuccess, anySuccess, firstCompletion }
}
```

`race` and `raceAll` wait for a success, so one early failure does not defeat another branch that can still succeed. If no branch succeeds, they fail with the collected failure reasons. `raceFirst` accepts the earliest completion, including failure, and the losing work is interrupted and cleaned up before the race returns.

## Inspecting the winner

```ts
function inspectRace() {
  const winners: number[] = []
  const outcome = Effect.result(Effect.raceFirst(
    () => { throw "primary offline" },
    () => {
      Effect.sleep("1 second")
      return "backup response"
    },
    { onWinner: ({ index }) => { winners.push(index) } }
  ))
  if (Result.isFailure(outcome)) {
    Effect.logWarning("first completion failed", outcome.failure)
  }
  return { outcome, winners }
}
```

The synchronous `onWinner` callback observes the winning fiber and index, zero for the first branch and one for the second. It does not replace the winning result and cannot run an Effeck operation that pauses. `Effect.result` captures expected failures as data; use `Effect.exit` when defects and interruption must also be represented.

## Coming from Effect

`Effect.all([task(1), task(2)])` in TypeScript becomes `Effect.all([() => task(1), () => task(2)])` in Effeck. Calling `task(1)` before passing it to the combinator would already run the work.
