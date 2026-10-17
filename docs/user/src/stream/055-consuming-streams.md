# Consuming Streams

A consumer runs a Stream and returns one result. Inside an Effeck function, calls such as `Stream.runCollect` wait for consumption and give you the result directly. An unbounded source needs a stopping condition before a consumer that waits for its end.

## Collecting elements

```ts
function collectLabels() {
  return Stream.runCollect(Stream.make("north", "south", "west"))
}
// Effect<Array<string>, never, never>
```

`runCollect` returns an array in emission order. It retains every element, so use it for finite results that fit in memory. It fails if the source fails; it does not return a partial array as a successful result.

## Executing per-element effects

```ts
function announce() {
  Stream.runForEach(Stream.make("build", "test", "package"), (stage) => {
    Effect.log(`starting ${stage}`)
    Effect.sleep("20 millis")
  })
}
// Effect<void, never, never>
```

`runForEach` runs the callback sequentially for each element and discards its result. Callback errors and service requirements join those of the source. For concurrent preparation, use `Stream.mapEffect` with a concurrency limit before consuming.

## Stopping conditionally

```ts
function reachBudget() {
  let total = 0
  Stream.runForEachWhile(Stream.make(3, 5, 8, 13), (amount) => {
    total += amount
    return total < 10
  })
  return total // 16
}
```

The callback returns `true` to continue or `false` to stop. The element that returns `false` has already been consumed and its callback has run. Stopping closes the source and runs its finalizers; state declared inside the function is fresh for each call.

## Folding into an accumulator

```ts
function totalBytes() {
  return Stream.runFold(Stream.make(256, 512, 128), () => 0, (total, n) => total + n)
}
// Returns 896.

function totalWithProgress() {
  return Stream.runFoldEffect(Stream.make(256, 512, 128), () => 0, (total, n) => {
    const next = total + n
    Effect.log(`processed ${next} bytes`)
    return next
  })
}
```

The initial-state callback runs for each consumption, which also supports fresh mutable accumulators. `runFold` requires a synchronous, non-failing reducer; `runFoldEffect` accepts Effeck work. Both wait for the source to finish; use `runForEachWhile` when the accumulator should trigger early termination.

## Running a sink

```ts
function readHeader() {
  const source = Stream.make("version", "length", "body")
  return Stream.run(source, Sink.take<string>(2))
}
// Returns ["version", "length"].
```

A Sink packages a reusable consumption policy. `Stream.run` combines the source's and sink's failures and requirements, then returns the sink's result. A sink can finish before the source ends; see [Leftovers](../sink/062-leftovers.md) for elements it has already pulled but not used.
