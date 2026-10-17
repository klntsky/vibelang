# Fibers

A fiber is a running computation with a result and a lifetime. Effeck uses Effect's lightweight fibers to overlap waiting work, inspect completion, and cancel safely. Fibers provide concurrency on the runtime's scheduler; they do not turn JavaScript computation into parallel CPU execution.

## Forking and awaiting

```ts
function buildDashboard() {
  const chart = fork {
    sleep("30 millis")
    return "chart"
  }
  logInfo("preparing dashboard frame")
  return await chart
}
// string!!
```

`fork` starts a child fiber and returns its handle, here a `Fiber<string>`, without waiting for its result. `await` waits for the child and returns its value, or propagates its failure. Unlike an ordinary call, the fork lets the parent continue while its child is waiting.

`fork loadChart()` forks one call, and `fork { ... }` forks a block. The call form, `fork(loadChart())`, means the same. `await` never waits for a call: a call already runs and waits where it stands, so `await loadChart()` is an error (EK4601).

## Awaiting completion as data

```ts
function inspectChild() {
  const child = fork { throw "render failed" }
  const exit = Fiber.await(child)
  if (exit.isFailure()) {
    logError("renderer stopped", exit.cause)
  }
  return exit
}
```

`Fiber.await` succeeds with an `Exit`, so it does not rethrow the child's failure into the caller. The exit also represents defects and interruption. Use `await child` when failure should propagate and `Fiber.await(child)` when the caller needs to decide what the outcome means.

## Handling asynchronous interruption

```ts
function stopWorker() {
  const worker = fork {
    try {
      sleep("1 minute")
    } finally {
      logInfo("worker finalizer")
    }
  }
  sleep("10 millis")
  worker.interrupt()
  logInfo("worker has terminated")
}
```

Cancellation does not require a hand-written polling loop. Effect delivers it at runtime boundaries, defers it in uninterruptible regions, and runs cleanup before `interrupt` returns. A long plain JavaScript calculation still occupies the host thread until it returns or reaches an Effect operation.

When the caller needs to continue during cleanup, fork the interruption, `fork worker.interrupt()`, and await that fiber before its owner exits. Avoid immediate unsafe runtime methods when cleanup ordering matters.

## Combining results

```ts
function combine() {
  const title = fork { return "Inventory" }
  const rows = fork {
    sleep("20 millis")
    return 18
  }
  const [name, count] = await [title, rows]
  return `${name}: ${count}`
}
// string!!
```

`await` on an array or an object waits for every fiber in it and keeps the shape. Both children above already run concurrently before the `await` begins. When one fiber fails, `await` interrupts the others still running and propagates the failure, as `all` fails fast. For an expected-failure fallback, put `try` around the `await` and recover in `catch`; defects and interruption still propagate.

## Selecting the first event

```ts
function nextEvent(jobs: Queue.Queue<string>, shutdown: Deferred.Deferred<void>) {
  select {
    case const job = Queue.take(jobs): return `job: ${job}`
    case Deferred.await(shutdown): return "shutdown"
    case sleep("30 seconds"): return "idle"
  }
}
// string!!
```

`select` starts each case in its own fiber and runs the statements of the first to finish; the other cases are interrupted. A case's expression is a program, passed rather than run, and `case const job = ...:` binds the winner's value. If the first case to finish failed, the select fails with that failure.

Of cases that are ready at once, the first written wins. A `default:` clause runs when no case finishes while the cases start, which turns `select` into a poll. `break` leaves the select, as it leaves a `switch`; `continue` and `return` keep their meaning in the enclosing loop and function.

## Running a loop body concurrently

```ts
function fetchPage(id: string) {
  sleep("5 millis")
  return `page ${id}`
}

function printAll(ids: ReadonlyArray<string>) {
  for concurrent(8) (const id of ids) {
    if (id === "") continue
    logInfo(fetchPage(id))
  }
}
```

`for concurrent` runs its body for each item in its own fiber and waits for them all before the loop ends; `for concurrent(8)` runs at most eight at once, and `for concurrent (...)` sets no limit. A failure interrupts the items still running and fails the loop. `continue` ends one item. Because each item runs in its own fiber, a `break` that would leave the loop (EK4605) and a `return` (EK4606) are errors; throw to stop every item. Use `forEach` when you need the results.

## Supervising child lifetimes

```ts
function request() {
  {
    const progress = fork {
      try {
        sleep("1 minute")
      } finally {
        logInfo("progress reporter stopped")
      }
    }
    sleep("20 millis")
    // The nested block owns progress, even without an await.
  }
  logInfo("request finishing")
}
```

The Effeck `fork` belongs to its enclosing block. Leaving that block interrupts unfinished children and waits for their cleanup. `Effect.forkChild` remains available when ownership should follow the parent fiber instead of the lexical block.

Forked fibers inherit the current services and tracing context. Control-operation handlers belong to one fiber, so handle those operations inside each child that performs them.

## Selecting scoped or detached lifetimes

```ts
function inCallerScope(scope: Scope) {
  const background = forkIn(scope, sleep("1 minute"))
  sleep("20 millis")
  // Closing scope interrupts background.
  return background
}

function detachedExample() {
  const independent = detach(sleep("50 millis"))
  await independent
}
```

`forkIn(scope, program)` attaches the child to a scope you hold, so it lives until that scope closes. `detach(program)` ties the child to nothing: it survives its parent and needs an explicit shutdown strategy; awaiting it above keeps the example bounded. `Effect.forkScoped` attaches the child to the caller's scope and leaves `Scope` in the caller's requirements until a scope is supplied.

## Understanding execution scheduling

```ts
function waitForReady() {
  const ready = Deferred.make<void>()
  const worker = fork {
    logInfo("worker initialized")
    ready.succeed(undefined)
    sleep("20 millis")
  }
  await ready
  logInfo("initialization is complete")
  await worker
}
```

Forking schedules a child; it does not prove the child has reached a particular statement. `Effect.yieldNow()` gives other work a chance to run, but it is not a readiness guarantee. Use a `Deferred`, as above, when ordering matters; `await ready` waits until the child completes it. Sleeping for an estimated startup time leaves a race.
