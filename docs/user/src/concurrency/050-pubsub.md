# PubSub

A `PubSub` broadcasts values to independent subscribers. Each active subscriber can receive the same published value, making it useful for notifications and live event feeds. Use a Queue instead when workers should divide messages among themselves.

## Broadcasting to independent subscribers

```vibe
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
```

Subscribe before publishing a message that must be received. Without replay configuration, later subscribers receive future messages rather than past ones. Each subscription has its own read position; reading from one does not consume the other subscriber's copy.

## Selecting capacity and overflow behavior

```vibe
function makeHubs() {
  return {
    reliable: PubSub.bounded<string>(64),
    bestEffort: PubSub.dropping<string>(64),
    latest: PubSub.sliding<string>(64),
    unlimited: PubSub.unbounded<string>()
  }
}
```

A bounded hub backpressures publishers when its buffer is full, so a slow subscriber can delay publication. Dropping rejects new values when full and reports `false`; sliding removes old values to retain newer ones, so slow subscribers can miss events. Unbounded hubs avoid capacity waits at the cost of memory growth when subscribers fall behind.

## Managing scoped subscriptions

```vibe
function listen(hub: PubSub.PubSub<string>) {
  using subscription = PubSub.subscribe(hub)
  const message = PubSub.take(subscription)
  logInfo("notification", message)
}
```

`subscribe` acquires a scoped subscription, and `using` releases it when the block exits on success, failure, or interruption. Releasing a subscriber also removes its pressure on retained messages. Acquiring without `using` leaves `Scope` in the caller's requirements, allowing the caller's scope to own the subscription.

In Effect 4, consume the returned `PubSub.Subscription` with `PubSub.take` and the other PubSub taking functions.

## Publishing batches

```vibe
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
```

`publishAll` publishes a collection using the hub's overflow strategy and reports whether all values were published. A bounded batch can suspend while subscribers make room. Run a large producer concurrently with its consumers when the batch exceeds available capacity.

## Inspecting size and capacity

```vibe
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
```

Capacity is a plain value, while size and fullness queries run Effects. Size measures messages retained by the hub, not a sum of copies across subscribers. These are snapshots and cannot guarantee that a later publication will find space.

## Ending subscriptions and shutting down

```vibe
function ownHub() {
  const hub = PubSub.bounded<string>(8)
  try {
    {
      using subscriber = PubSub.subscribe(hub)
      PubSub.publish(hub, "last update")
      logInfo(PubSub.take(subscriber))
    } // This subscription is released here.
  } finally {
    PubSub.shutdown(hub)
  }
  PubSub.awaitShutdown(hub)
  return PubSub.isShutdown(hub)
}
```

Closing one subscription leaves the hub and other subscriptions active. Shutting down the hub terminates its subscriptions and releases blocked operations. `awaitShutdown` waits for the shutdown signal, and `isShutdown` reads that state; keep hub shutdown in the owner's cleanup path.
