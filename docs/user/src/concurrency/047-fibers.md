# Fibers

A fiber is a running computation with a result and a lifetime. Effeck uses Effect's lightweight fibers to overlap waiting work, inspect completion, and cancel safely. Fibers provide concurrency on the runtime's scheduler; they do not turn JavaScript computation into parallel CPU execution.

## Forking and joining

```ts
function buildDashboard() {
  const chart = fork(() => {
    Effect.sleep("30 millis")
    return "chart"
  })
  Effect.logInfo("preparing dashboard frame")
  return Fiber.join(chart)
}
// Effect<string, never, never>
```

`fork` starts a child and returns its handle without waiting for its result. `Fiber.join` waits and returns the success value or propagates the child's failure. Unlike an ordinary call, the fork lets the parent continue while its child is waiting.

## Awaiting completion as data

```ts
function inspectChild() {
  const child = fork(() => { throw "render failed" })
  const exit = Fiber.await(child)
  if (Exit.isFailure(exit)) {
    Effect.logError("renderer stopped", exit.cause)
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
      Effect.sleep("1 minute")
    } finally {
      Effect.logInfo("worker finalizer")
    }
  })
  Effect.sleep("10 millis")
  Fiber.interrupt(worker)
  Effect.logInfo("worker has terminated")
}
```

Cancellation does not require a hand-written polling loop. Effect delivers it at runtime boundaries, defers it in uninterruptible regions, and runs cleanup before `Fiber.interrupt` returns. A long plain JavaScript calculation still occupies the host thread until it returns or reaches an Effect operation.

When the caller needs to continue during cleanup, start `Fiber.interrupt(worker)` in another owned fiber and join that cancellation fiber before its owner exits. Avoid immediate unsafe runtime methods when cleanup ordering matters.

## Combining results

```ts
function combine() {
  const title = fork(() => "Inventory")
  const rows = fork(() => {
    Effect.sleep("20 millis")
    return 18
  })
  const both = Effect.all([
    Fiber.join(title),
    Fiber.join(rows)
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
        Effect.sleep("1 minute")
      } finally {
        Effect.logInfo("progress reporter stopped")
      }
    })
    Effect.sleep("20 millis")
    // The nested block owns progress, even without an explicit join.
  }
  Effect.logInfo("request finishing")
}
```

The Effeck `fork` belongs to its enclosing block. Leaving that block interrupts unfinished children and waits for their cleanup. `Effect.forkChild` remains available when ownership should follow the parent fiber instead of the lexical block.

Forked fibers inherit the current services and tracing context. Control-operation handlers belong to one fiber, so handle those operations inside each child that performs them.

## Selecting scoped or detached lifetimes

```ts
function startInCallerScope() {
  return Effect.forkScoped(Effect.sleep("1 minute"))
}
// Scope remains in this function's requirements.

function scopedExample() {
  Effect.scoped(() => {
    const background = startInCallerScope()
    Effect.sleep("20 millis")
    // Closing this scope interrupts background.
  })
}

function detachedExample() {
  const independent = Effect.forkDetach(() => Effect.sleep("50 millis"))
  Fiber.join(independent)
}
```

`forkScoped` attaches the child to the supplied runtime scope and leaves `Scope` in the caller's requirements until a scope is supplied. `Effect.forkIn(program, scope)` selects an explicit scope. A detached fiber survives its parent and needs an explicit shutdown strategy; joining it above keeps this example bounded.

## Understanding execution scheduling

```ts
function waitForReady() {
  const ready = Deferred.make<void>()
  const worker = fork(() => {
    Effect.logInfo("worker initialized")
    Deferred.succeed(ready, undefined)
    Effect.sleep("20 millis")
  })
  Deferred.await(ready)
  Effect.logInfo("initialization is complete")
  Fiber.join(worker)
}
```

Forking schedules a child; it does not prove the child has reached a particular statement. `Effect.yieldNow()` gives other work a chance to run, but it is not a readiness guarantee. Use a `Deferred`, as above, when ordering matters; sleeping for an estimated startup time leaves a race.
