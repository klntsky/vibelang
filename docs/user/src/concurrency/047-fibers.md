# Fibers

A fiber is a running computation with a result and a lifetime. Effeck uses Effect's lightweight fibers to overlap waiting work, inspect completion, and cancel safely. Fibers provide concurrency on the runtime's scheduler; they do not turn JavaScript computation into parallel CPU execution.

## Forking and joining

```ts
function buildDashboard() {
  const chart = fork(() => {
    sleep("30 millis")
    return "chart"
  })
  logInfo("preparing dashboard frame")
  return chart.join()
}
// string!!
```

`fork` starts a child and returns its handle without waiting for its result. `Fiber.join` waits and returns the success value or propagates the child's failure. Unlike an ordinary call, the fork lets the parent continue while its child is waiting.

## Awaiting completion as data

```ts
function inspectChild() {
  const child = fork(() => { throw "render failed" })
  const exit = Fiber.await(child)
  if (exit.isFailure()) {
    logError("renderer stopped", exit.cause)
  }
  return exit
}
```

`Fiber.await` succeeds with an `Exit`, so it does not rethrow the child's failure into the caller. The exit also represents defects and interruption. Use `join` when failure should propagate and `await` when the caller needs to decide what the outcome means.

## Handling asynchronous interruption

```ts
function stopWorker() {
  const worker = fork(() => {
    try {
      sleep("1 minute")
    } finally {
      logInfo("worker finalizer")
    }
  })
  sleep("10 millis")
  worker.interrupt()
  logInfo("worker has terminated")
}
```

Cancellation does not require a hand-written polling loop. Effect delivers it at runtime boundaries, defers it in uninterruptible regions, and runs cleanup before `Fiber.interrupt` returns. A long plain JavaScript calculation still occupies the host thread until it returns or reaches an Effect operation.

When the caller needs to continue during cleanup, start `Fiber.interrupt(worker)` in another owned fiber and join that cancellation fiber before its owner exits. Avoid immediate unsafe runtime methods when cleanup ordering matters.

## Combining results

```ts
function combine() {
  const title = fork(() => "Inventory")
  const rows = fork(() => {
    sleep("20 millis")
    return 18
  })
  const both = all([
    title.join(),
    rows.join()
  ], { concurrency: 2 })
  return both
}
```

Compose programs that join fibers rather than combining handles themselves. Both children above already run concurrently before their joins begin. For an expected-failure fallback, use `try` around the first `join` and join a second child in `catch`; defects and interruption still propagate.

## Supervising child lifetimes

```ts
function request() {
  {
    const progress = fork(() => {
      try {
        sleep("1 minute")
      } finally {
        logInfo("progress reporter stopped")
      }
    })
    sleep("20 millis")
    // The nested block owns progress, even without an explicit join.
  }
  logInfo("request finishing")
}
```

The Effeck `fork` belongs to its enclosing block. Leaving that block interrupts unfinished children and waits for their cleanup. `Effect.forkChild` remains available when ownership should follow the parent fiber instead of the lexical block.

Forked fibers inherit the current services and tracing context. Control-operation handlers belong to one fiber, so handle those operations inside each child that performs them.

## Selecting scoped or detached lifetimes

```ts
function startInCallerScope() {
  return Effect.forkScoped(sleep("1 minute"))
}
// Scope remains in this function's requirements.

function scopedExample() {
  Effect.scoped(() => {
    const background = startInCallerScope()
    sleep("20 millis")
    // Closing this scope interrupts background.
  })
}

function detachedExample() {
  const independent = Effect.forkDetach(() => sleep("50 millis"))
  independent.join()
}
```

`forkScoped` attaches the child to the supplied runtime scope and leaves `Scope` in the caller's requirements until a scope is supplied. `Effect.forkIn(program, scope)` selects an explicit scope. A detached fiber survives its parent and needs an explicit shutdown strategy; joining it above keeps this example bounded.

## Understanding execution scheduling

```ts
function waitForReady() {
  const ready = Deferred.make<void>()
  const worker = fork(() => {
    logInfo("worker initialized")
    ready.succeed(undefined)
    sleep("20 millis")
  })
  Deferred.await(ready)
  logInfo("initialization is complete")
  worker.join()
}
```

Forking schedules a child; it does not prove the child has reached a particular statement. `Effect.yieldNow()` gives other work a chance to run, but it is not a readiness guarantee. Use a `Deferred`, as above, when ordering matters; sleeping for an estimated startup time leaves a race.
