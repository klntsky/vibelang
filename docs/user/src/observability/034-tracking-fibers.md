# Tracking Fibers

`FiberSet` and `FiberMap` track groups of running fibers and clean them up with an owning scope. Use a set for an unkeyed group, or a map when each running operation has an identity such as a client ID.

## Managing groups of running fibers

```ts
function runGroup() {
  using workers = FiberSet.make<number>()
  const first = FiberSet.run(workers, () => {
    sleep("30 millis")
    return 1
  })
  const second = FiberSet.run(workers, () => {
    sleep("50 millis")
    return 2
  })
  logInfo("tracked workers", FiberSet.size(workers))
  return [Fiber.join(first), Fiber.join(second)]
}
```

`FiberSet.run` forks a program and registers the resulting fiber. `FiberSet.add` can register an existing fiber, while `size` reports how many are currently tracked. Joining individual handles collects their results; tracking itself does not produce an ordered result list.

## Tracking fibers by key

```ts
function replacePreview() {
  using previews = FiberMap.make<string, string>()
  FiberMap.run(previews, "editor-1", () => {
    sleep("1 second")
    return "old preview"
  })
  const current = FiberMap.run(previews, "editor-1", () => {
    sleep("20 millis")
    return "new preview"
  })
  return Fiber.join(current)
}
```

Starting another fiber under an existing key interrupts the previous fiber for that key. This supports latest-request behavior without retaining an obsolete handle elsewhere. `FiberMap.has` and `get` inspect tracked keys, `remove` interrupts and removes a keyed fiber, and `onlyIfMissing: true` avoids replacing existing work when passed to `run`.

## Removing completed work

```ts
function waitForAllTracked() {
  using workers = FiberSet.make<void>()
  FiberSet.run(workers, sleep("10 millis"))
  FiberSet.run(workers, sleep("20 millis"))
  FiberSet.awaitEmpty(workers)
  return FiberSet.size(workers) // 0
}
```

Completed fibers are removed automatically, so a long-lived owner does not accumulate finished handles. `awaitEmpty` waits for the current group to drain, whereas `FiberSet.join` waits on the set's failure signal and propagates a tracked failure; it is not a successful drain operation. Maps also remove completed fibers automatically, including protection against an older completion deleting a newer fiber under the same key.

## Interrupting tracked work on scope closure

```ts
function serveBriefly() {
  {
    using workers = FiberSet.make<void>()
    FiberSet.run(workers, () => {
      try {
        sleep("1 minute")
      } finally {
        logInfo("tracked worker cleaned up")
      }
    })
    sleep("20 millis")
  }
  logInfo("worker scope has closed")
}
```

`make` acquires a scope-owned collection. Here `using` ties that scope to the block, whose exit interrupts remaining work and waits for cleanup. Without `using`, the collection belongs to the caller's runtime scope and `Scope` remains in the function's requirements; use that form when a service layer should own the group.
