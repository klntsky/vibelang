# Latch

A latch is a reusable gate for fibers. A closed latch makes callers wait; an open latch lets them pass. Use one when many operations must wait for a condition such as completed initialization.

## Waiting at a reusable gate

```ts
import { Fiber, Latch } from "effect"

function initializeThenWork() {
  const ready = Latch.make()
  const worker = fork(() => {
    Latch.await(ready)
    return "processing enabled"
  })
  Effect.sleep("20 millis")
  Latch.open(ready)
  return Fiber.join(worker)
}
```

`Latch.await` waits until the latch permits the fiber to proceed. Waiting does not block other fibers, and an already open latch returns immediately. `Latch.whenOpen` can wrap a program when the wait and action read better as one expression.

## Opening and closing

```ts
import { Fiber, Latch } from "effect"

function reopenGate() {
  const gate = Latch.make(true)
  Latch.await(gate) // Passes immediately.
  Latch.close(gate)
  const waiting = fork(() => Latch.whenOpen(gate, () => "allowed again"))
  Latch.open(gate)
  return Fiber.join(waiting)
}
```

Opening releases current waiters and allows future callers through. Closing makes future callers wait again; it does not cancel work that already passed the gate. `Latch.release` releases the currently waiting fibers without leaving the latch permanently open, so later arrivals can still wait.

## Choosing the initial state

```ts
import { Latch } from "effect"

function createGates() {
  const starting = Latch.make()      // Closed by default.
  const paused = Latch.make(false)   // Explicitly closed.
  const active = Latch.make(true)    // Open from the start.
  return { starting, paused, active }
}
```

Create the latch once and share it with all operations controlled by the same condition. Opening is persistent until a close, so a caller arriving after initialization does not miss the signal. A latch carries no result value or typed failure; use a Deferred when waiters need either of those.

## Choosing a latch or a permit pool

```ts
import { Latch, Semaphore } from "effect"

function gatedExclusiveWork() {
  const initialized = Latch.make(true)
  const exclusive = Semaphore.make(1)
  Latch.await(initialized)
  return pipe(
    () => Effect.logInfo("updating shared device"),
    Semaphore.withPermits(exclusive, 1)
  )
}
```

The latch answers whether callers may proceed at all. The semaphore limits how many may remain inside a protected operation at once. An open latch admits every caller and does not provide mutual exclusion; share the semaphore too when callers must take turns.
