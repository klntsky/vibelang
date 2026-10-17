# SubscriptionRef

A `SubscriptionRef` combines shared state with a stream of changes. Use it for observable application state where each subscriber needs a current snapshot followed by later updates. Its updates support the same serialized, effectful operations as `SynchronizedRef`.

## Combining state with subscriptions

```ts
function makeProgress() {
  return SubscriptionRef.make({ completed: 0, total: 12 })
}

function advance(progress: SubscriptionRef.SubscriptionRef<{
  readonly completed: number
  readonly total: number
}>) {
  SubscriptionRef.update(progress, (state) => ({
    ...state,
    completed: state.completed + 1
  }))
}
```

Creating the reference establishes its initial state. Updating through `SubscriptionRef` commits the value and publishes the change to subscribers. Keep values immutable so observers receive meaningful snapshots.

## Reading and updating observable state

```ts
function renameStatus() {
  const status = SubscriptionRef.make("starting")
  const before = SubscriptionRef.get(status)
  SubscriptionRef.set(status, "running")
  const after = SubscriptionRef.updateAndGet(status, (s) => s.toUpperCase())
  return { before, after }
}
// Returns { before: "starting", after: "RUNNING" }.
```

`get` reads without subscribing. `set`, `update`, and `modify` change the value; their effectful variants can compute replacements while holding the update lock. A read outside an update callback is a snapshot, not a reservation of the value for a later write.

## Consuming current and future values

```ts
function currentSnapshot() {
  const count = SubscriptionRef.make(7)
  const changes = SubscriptionRef.changes(count)
  return Stream.runCollect(Stream.take(changes, 1)) // [7]
}
```

`changes` builds a Stream; it does not begin listening until that stream runs. Each run first emits the value current at subscription time, then receives future updates. Consume it with Stream operators such as `take` and `runForEach`, or iterate it directly with `for...of`. Leaving the loop closes the stream and its subscription.

## Coordinating concurrent subscribers

```ts
function observeTwoClients() {
  const count = SubscriptionRef.make(0)
  const firstReady = Deferred.make<void>()
  const secondReady = Deferred.make<void>()

  function client(ready: Deferred.Deferred<void>) {
    return Stream.runForEach(
      Stream.take(SubscriptionRef.changes(count), 2),
      (value) => {
        Effect.logInfo("observed", value)
        Deferred.succeed(ready, undefined)
      }
    )
  }

  const first = fork(() => client(firstReady))
  const second = fork(() => client(secondReady))
  Deferred.await(firstReady)
  Deferred.await(secondReady)
  SubscriptionRef.set(count, 1)
  Fiber.join(first)
  Fiber.join(second)
}
```

The readiness signals come from each subscriber's first stream element, so both subscriptions exist before the update. Each client observes zero and then one, and its `take(2)` ends its subscription. A sleep or one scheduler yield does not establish that ordering; use explicit readiness when every observer must see a particular change.
