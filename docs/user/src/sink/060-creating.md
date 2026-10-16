# Creating Sinks

Choose a Sink for the result you need, then run it with `Stream.run`. Constructors stay reusable: state supplied by an initializer is fresh on each run. This page uses [trusted Sink builders](059-introduction.md) and keeps effectful callbacks as ordinary EffectScript functions.

## Basic consumers and endpoint values

```ts
import { Sink, Stream } from "effect"

function endpoints() {
  const source = Stream.make("opening", "middle", "closing")
  return {
    first: Stream.run(source, Sink.head<string>()),
    last: Stream.run(source, Sink.last<string>()),
    prefix: Stream.run(source, Sink.take<string>(2))
  }
}
// first: Some("opening"), last: Some("closing")
// prefix: ["opening", "middle"]
```

`head` stops after the first element; `last` reads to the end. Both return `Option.none()` on empty input. `take(n)` returns up to `n` elements and completes early when it reaches the limit, preserving any unused suffix of its current input batch as leftovers.

## Counting, summing, draining, and timing

```ts
import { Sink, Stream } from "effect"

function summarize() {
  const source = Stream.make(6, 10, 14)
  return {
    count: Stream.run(source, Sink.count), // 3
    sum: Stream.run(source, Sink.sum),     // 30
    duration: Stream.run(source, Sink.timed)
  }
}

function drainObserved() {
  const source = Stream.tap(Stream.make("ready", "done"), (s) => Effect.log(s))
  Stream.run(source, Sink.drain)
}
```

These sinks consume the whole source. `drain` discards elements but still executes source effects; `timed` also drains and returns a `Duration.Duration`. `Sink.withDuration(otherSink)` returns `[result, duration]` when both the other sink's result and elapsed time are useful. Each field in `summarize` runs a separate consumption.

## Per-element work

```ts
import { Sink, Stream } from "effect"

const print = Sink.forEach((line: string) => {
  Effect.log(`record: ${line}`)
  Effect.sleep("10 millis")
})

function writeRecords() {
  Stream.run(Stream.make("created", "approved"), print)
}
```

`forEach` runs work sequentially and returns `void`. Its callback's expected failures and services become the sink's `E` and `R`. `forEachWhile` stops when its effectful callback returns `false`; `forEachArray` works on internal batches.

## Successful and failed sinks

```ts
import { Sink, Stream } from "effect"

class Rejected extends TaggedError("Rejected")<{}> {}

function immediateResult() {
  return Stream.run(Stream.make(40, 50), Sink.succeed("already complete"))
}

function rejectInput() {
  return Stream.run(Stream.make(40, 50), Sink.fail(new Rejected({})))
}
// Effect<never, Rejected, never>
```

`succeed` and `fail` finish without requesting source elements. Construction itself does not succeed or fail a running program; that happens when the sink runs. `Sink.fromEffect(() => work())` creates the same kind of immediate sink from an EffectScript program.

## Collecting by size or condition

```ts
import { Sink, Stream } from "effect"

function collectSamples() {
  const source = Stream.make(8, 5, -1, 13)
  return {
    all: Stream.run(source, Sink.collect<number>()),
    firstTwo: Stream.run(source, Sink.take<number>(2)),
    positivePrefix: Stream.run(source, Sink.takeWhile((n: number) => n > 0))
  }
}
// all: [8, 5, -1, 13], firstTwo: [8, 5], positivePrefix: [8, 5]
```

`collect` gathers all inputs into an array. `takeWhile` stops at the first rejected element and leaves it in the batch remainder; it does not skip it and continue searching. `takeUntil` includes the matching element before stopping, and the corresponding `takeWhileEffect` and `takeUntilEffect` variants accept effectful predicates.

## Uniqueness and key limits

```ts
import { Sink, Stream } from "effect"

const unique = Sink.reduce(() => new Set<string>(), (set, value: string) => set.add(value))
const threeUnique = Sink.reduceWhile(
  () => new Set<string>(),
  (set) => set.size < 3,
  (set, value: string) => set.add(value)
)

function collectUnique() {
  return Stream.run(Stream.make("oak", "oak", "ash", "elm", "pine"), threeUnique)
}
// Set containing "oak", "ash", "elm".
```

Use a native Set when JavaScript value equality is appropriate. A size limit counts unique values, so duplicates do not consume another slot. The initializer creates a fresh Set on every run, avoiding shared mutable sink state.

```ts
import { Sink, Stream } from "effect"

type Reading = { readonly station: string; readonly count: number }

function addReading(totals: Map<string, number>, reading: Reading) {
  totals.set(reading.station, (totals.get(reading.station) ?? 0) + reading.count)
  return totals
}

const allStations = Sink.reduce(() => new Map<string, number>(), (map, reading: Reading) => addReading(map, reading))
const twoStations = Sink.reduceWhile(
  () => new Map<string, number>(),
  (map) => map.size < 2,
  (map, reading: Reading) => addReading(map, reading)
)

function summarizeStations() {
  return Stream.run(Stream.make(
    { station: "hill", count: 4 },
    { station: "hill", count: 3 },
    { station: "lake", count: 8 },
    { station: "hill", count: 9 }
  ), twoStations)
}
// Map with "hill" -> 7 and "lake" -> 8.
```

The reducer defines both the grouping key and the merge rule. This limited sink stops immediately after reaching two keys, so even later updates for existing keys remain unread. It limits the consumed prefix, rather than scanning all input while rejecting new keys.

## Folding with termination and count limits

```ts
import { Sink, Stream } from "effect"

const total = Sink.reduce(() => 0, (sum, n: number) => sum + n)
const budget = Sink.fold(
  () => 0,
  (sum) => sum < 20,
  (sum, n: number) => {
    Effect.log(`adding ${n}`)
    return sum + n
  }
)
const firstThree = Sink.foldUntil(() => 0, 3, (sum, n: number) => sum + n)

function foldSamples() {
  const source = Stream.make(7, 9, 12, 15)
  return {
    all: Stream.run(source, total),       // 43
    budget: Stream.run(source, budget),   // 28
    first: Stream.run(source, firstThree) // 28
  }
}
```

`reduce` uses a pure reducer and consumes to the end; `reduceEffect` supports Effect work. `fold` accepts an effectful step and checks its continuation predicate after each consumed element. The element that reaches or crosses the limit is included. `foldUntil` instead stops after a maximum number of inputs or at source completion.

## Folding by weight

```ts
import { Sink, Stream } from "effect"

type Packet = { readonly bytes: number }
type Batch = { readonly packets: ReadonlyArray<Packet>; readonly bytes: number }

const weighted = Sink.fold<Batch, Packet>(
  () => ({ packets: [], bytes: 0 }),
  (batch) => batch.bytes < 100,
  (batch, packet) => ({
    packets: [...batch.packets, packet],
    bytes: batch.bytes + packet.bytes
  })
)
const packets = Sink.map(weighted, (batch) => batch.packets)

function firstBatch() {
  return Stream.run(Stream.make({ bytes: 60 }, { bytes: 55 }, { bytes: 10 }), packets)
}
// Returns the 60-byte and 55-byte packets.
```

The state tracks total cost instead of element count. This rule stops after reaching the threshold, so the final element can exceed the target; it is not a strict maximum-size packing algorithm. `Stream.transduce(source, packets)` repeats the sink to emit successive batches and feeds its leftovers into the next run. A sink that consumes nothing can produce an empty final result at end of input, so account for that in batch consumers.
