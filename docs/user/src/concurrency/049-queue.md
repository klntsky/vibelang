# Queue

A `Queue` passes values between fibers, delivering each offered value to one consumer. Use it to separate producers from workers and to apply backpressure when workers cannot keep up.

## Choosing an overflow strategy

```ts
function chooseQueues() {
  const jobs = Queue.bounded<string>(32)
  const optionalEvents = Queue.dropping<string>(32)
  const recentPositions = Queue.sliding<number>(32)
  const unrestricted = Queue.unbounded<string>()
  return { jobs, optionalEvents, recentPositions, unrestricted }
}
```

A bounded queue suspends offers when full until consumers free space. A dropping queue rejects new values when full, while a sliding queue discards the oldest buffered values to accept new ones. An unbounded queue imposes no capacity limit, so memory use grows if producers stay ahead of consumers.

## Offering values and batches

```ts
function offerWithBackpressure() {
  const queue = Queue.bounded<number>(1)
  Queue.offer(queue, 10)
  const secondOffer = fork(Queue.offer(queue, 20))
  const first = Queue.take(queue)
  Fiber.join(secondOffer)
  const second = Queue.take(queue)
  return [first, second] // [10, 20]
}

function rejectedBatch() {
  const queue = Queue.dropping<number>(2)
  return Queue.offerAll(queue, [10, 20, 30]) // [30]
}
```

`offer` returns whether the value was accepted. `offerAll` returns the values it could not add, so an empty array means the whole batch was accepted. Fork a producer when the current fiber must consume values to free the space that producer needs; offering into a full bounded queue and only then taking from it would wait forever.

## Taking, polling, and draining values

```ts
function consumeBatches() {
  const queue = Queue.bounded<number>(8)
  Queue.offerAll(queue, [1, 2, 3, 4, 5, 6])
  const first = Queue.take(queue)           // 1
  const optional = Queue.poll(queue)       // Some(2)
  const pair = Queue.takeN(queue, 2)        // [3, 4]
  const batch = Queue.takeBetween(queue, 1, 4) // [5, 6]
  const empty = Queue.clear(queue)         // []
  return { first, optional, pair, batch, empty }
}
```

`take` waits for one value, `takeN` waits for an exact count, and `takeBetween` waits for its minimum then takes up to its maximum. In this version, `takeAll` waits for at least one value and returns the currently available nonempty batch. Use `poll` for an optional single value without waiting, or `clear` to drain buffered values without waiting for new ones.

## Querying queue state

```ts
function inspectQueue() {
  const queue = Queue.bounded<string>(2)
  Queue.offer(queue, "task")
  return {
    capacity: queue.capacity,
    size: Queue.size(queue),
    full: Queue.isFull(queue)
  }
}
// Returns { capacity: 2, size: 1, full: false }.
```

Size counts buffered messages, and capacity is the configured limit. These values are observations, not reservations; another fiber can change the queue immediately after a check. Call `offer` or `take` directly to coordinate instead of checking state first.

## Shutting down and awaiting closure

```ts
function finishNormally() {
  const queue = Queue.bounded<number, Cause.Done>(4)
  Queue.offerAll(queue, [7, 8])
  Queue.end(queue)
  const values = Queue.collect(queue)
  Queue.await(queue)
  return values // [7, 8]
}

function stopImmediately() {
  const queue = Queue.bounded<number>(4)
  const waiter = fork(() => {
    try {
      Queue.await(queue)
    } finally {
      Effect.logInfo("queue closed")
    }
  })
  Queue.shutdown(queue)
  return Fiber.await(waiter)
}
```

`end` stops new offers and lets buffered values drain before normal completion; `collect` consumes until that completion. `shutdown` immediately discards buffered values and interrupts waiting operations. `Queue.await` waits for the terminal state and preserves its outcome, so awaiting a shut-down queue is interrupted; inspect that child with `Fiber.await` rather than propagating it through `join`.

Queues do not automatically close at ordinary block exit. Have their owner call shutdown in `finally` when consumers and producers must stop with it.

## Restricting producer and consumer capabilities

```ts
function send(outbox: Queue.Enqueue<string>, message: string) {
  return Queue.offer(outbox, message)
}

function receive(inbox: Queue.Dequeue<string>) {
  return Queue.take(inbox)
}

function handoff() {
  const queue = Queue.bounded<string>(2)
  send(Queue.asEnqueue(queue), "ready")
  return receive(Queue.asDequeue(queue))
}
```

`Enqueue` gives a producer access to offering operations, while `Dequeue` gives a consumer access to taking operations. The adapters return the same underlying queue with narrower types. This restriction documents ownership and catches accidental use of the wrong side at compile time.
