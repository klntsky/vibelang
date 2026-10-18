# PubSub

A `PubSub` broadcasts values to independent subscribers. Each active subscriber can receive the same published value, making it useful for notifications and live event feeds. Use a Queue instead when workers should divide messages among themselves.

## Broadcasting to independent subscribers

```vibe
// run: Console.log(JSON.stringify(broadcast()))
function broadcast() {
  const hub = PubSub.bounded<string>(4)
  try {
    using display = PubSub.subscribe(hub)
    using audit = PubSub.subscribe(hub)
    PubSub.publish(hub, "deployment complete")
    return [PubSub.take(display), PubSub.take(audit)]
  } finally {
    PubSub.shutdown(hub)
  }
}
// Returns ["deployment complete", "deployment complete"].
// Output: ["deployment complete","deployment complete"]
```

Subscribe before publishing a message that must be received. Without replay configuration, later subscribers receive future messages rather than past ones. Each subscription has its own read position; reading from one does not consume the other subscriber's copy.

## Selecting capacity and overflow behavior

```vibe
// run: const hubs = makeHubs(); Console.log(PubSub.capacity(hubs.reliable), PubSub.capacity(hubs.bestEffort), PubSub.capacity(hubs.latest), PubSub.capacity(hubs.unlimited)); for (const hub of Object.values(hubs)) PubSub.shutdown(hub)
function makeHubs() {
  return {
    reliable: PubSub.bounded<string>(64),
    bestEffort: PubSub.dropping<string>(64),
    latest: PubSub.sliding<string>(64),
    unlimited: PubSub.unbounded<string>()
  }
}
// Output: 64 64 64 9007199254740991
```

A bounded hub backpressures publishers when its buffer is full, so a slow subscriber can delay publication. Dropping rejects new values when full and reports `false`; sliding removes old values to retain newer ones, so slow subscribers can miss events. Unbounded hubs avoid capacity waits at the cost of memory growth when subscribers fall behind.

## Managing scoped subscriptions

```vibe
// run: const hub = PubSub.bounded<string>(2); const listener = Effect.forkChild(() => listen(hub), { startImmediately: true }); PubSub.publish(hub, "ready"); Fiber.join(listener); PubSub.shutdown(hub)
function listen(hub: PubSub.PubSub<string>) {
  using subscription = PubSub.subscribe(hub)
  const message = PubSub.take(subscription)
  Console.log("notification", message)
}
// Output: notification ready
```

`subscribe` acquires a scoped subscription, and `using` releases it when the block exits on success, failure, or interruption. Releasing a subscriber also removes its pressure on retained messages. Acquiring without `using` leaves `Scope` in the caller's requirements, allowing the caller's scope to own the subscription.

In Effect 4, consume the returned `PubSub.Subscription` with `PubSub.take` and the other PubSub taking functions.

## Publishing batches

```vibe
// run: Console.log(JSON.stringify(batch()))
function batch() {
  const hub = PubSub.bounded<string>(3)
  try {
    using subscription = PubSub.subscribe(hub)
    const accepted = PubSub.publishAll(hub, ["queued", "started", "finished"])
    const messages = PubSub.takeAll(subscription)
    return { accepted, messages }
  } finally {
    PubSub.shutdown(hub)
  }
}
// Output: {"accepted":true,"messages":["queued","started","finished"]}
```

`publishAll` publishes a collection using the hub's overflow strategy and reports whether all values were published. A bounded batch can suspend while subscribers make room. Run a large producer concurrently with its consumers when the batch exceeds available capacity.

## Inspecting size and capacity

```vibe
// run: Console.log(JSON.stringify(inspect()))
function inspect() {
  const hub = PubSub.bounded<number>(5)
  try {
    using subscriber = PubSub.subscribe(hub)
    PubSub.publish(hub, 1)
    return {
      capacity: PubSub.capacity(hub),
      size: PubSub.size(hub),
      empty: PubSub.isEmpty(hub),
      full: PubSub.isFull(hub)
    }
  } finally {
    PubSub.shutdown(hub)
  }
}
// Output: {"capacity":5,"size":1,"empty":false,"full":false}
```

Capacity is a plain value, while size and fullness queries run Effects. Size measures messages retained by the hub, not a sum of copies across subscribers. These are snapshots and cannot guarantee that a later publication will find space.

## Ending subscriptions and shutting down

```vibe
// run: Console.log(ownHub())
function ownHub() {
  const hub = PubSub.bounded<string>(8)
  try {
    {
      using subscriber = PubSub.subscribe(hub)
      PubSub.publish(hub, "last update")
      Console.log(PubSub.take(subscriber))
    } // This subscription is released here.
  } finally {
    PubSub.shutdown(hub)
  }
  PubSub.awaitShutdown(hub)
  return PubSub.isShutdown(hub)
}
// Output: last update
// Output: true
```

Closing one subscription leaves the hub and other subscriptions active. Shutting down the hub terminates its subscriptions and releases blocked operations. `awaitShutdown` waits for the shutdown signal, and `isShutdown` reads that state; keep hub shutdown in the owner's cleanup path.
