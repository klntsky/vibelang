# Leftovers

Leftovers are elements a sink has already received in a batch but has not consumed when it completes. Preserve them when another sink should continue from that batch, or move them into the result for inspection. They are not a collection of every element remaining in the source.

## Collecting leftover elements

```effeck
function inspectRemainder() {
  const source = Stream.fromArray([21, 34, 55, 89])
  const sink = (Sink.take<number>(2)).mapEnd(([selected, leftover]) => [
    { selected, leftover: leftover ?? [] }
  ] as const)
  return source.run(sink)
}
// { selected: [21, 34], leftover: [55, 89] }
```

`Sink.mapEnd` receives `[result, optionalLeftoverBatch]` and returns another such tuple. Returning a one-element tuple moves the leftovers into the success result and leaves no separate leftover batch. `Stream.run` then returns that success result.

```effeck
function inspectSmallBatches() {
  const source = Stream.fromArray([21, 34, 55, 89]).rechunk(2)
  const sink = (Sink.head<number>()).mapEnd(([first, leftover]) => [
    { first, leftover: leftover ?? [] }
  ] as const)
  return source.run(sink)
}
// { first: Option.some(21), leftover: [34] }
```

Here `55` and `89` are in a batch the sink never requests. Changing internal batch boundaries therefore changes the leftovers you can inspect. Use `Stream.peel` when you need both the sink result and a live remainder stream, including input not yet requested; consume that remainder inside its Scope.

## Explicitly discarding leftovers

```effeck
function discardRemainder() {
  const sink = Sink.take<number>(2)
    .ignoreLeftover()
    .mapEnd(([selected, leftover]) => [
      { selected, leftover: leftover ?? [] }
    ] as const)
  return Stream.fromArray([21, 34, 55, 89]).run(sink)
}
// { selected: [21, 34], leftover: [] }
```

`ignoreLeftover` changes the sink's leftover type to `never` and drops its unused batch suffix. It does not drain the rest of the upstream source. In sink composition or repeated consumption, dropping leftovers can lose already-pulled elements, so use it only when that loss is intended.
