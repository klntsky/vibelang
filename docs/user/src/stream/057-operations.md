# Operations

Stream operators build a new description of how to process a source. Keep transformations that only compute values synchronous, and use the effectful variants when callbacks call services, fail, or wait. Stream consumption determines when those callbacks run.

## Observing and taking elements

```ts
import { Stream } from "effect"

const observed = Stream.tap(Stream.range(10, 18), (n) => Effect.log(`reading ${n}`))
const first = Stream.take(observed, 3)
const below = Stream.takeWhile(observed, (n) => n < 13)
const through = Stream.takeUntil(observed, (n) => n === 13)
const last = Stream.takeRight(observed, 2)

function readFirst() {
  return Stream.runCollect(first) // [10, 11, 12]
}
```

`tap` observes elements without replacing them. `takeWhile` excludes the first element that fails its predicate; `takeUntil` includes the first element that satisfies its predicate. `takeRight` needs the source to end before it can emit its final elements, so it does not suit an infinite source.

`takeWhileEffect` and `takeUntilEffect` accept effectful predicates. A tap or predicate that fails also fails the resulting stream.

## Async iteration and manual pulls

```ts
import { Stream } from "effect"

abstract class Exporter {
  abstract write(rows: AsyncIterable<string>): Effect<number, Error, never>
}

function exportRows(source: Stream.Stream<string>) {
  const rows = Stream.toAsyncIterableEffect(source)
  return Exporter.write(rows)
}
```

`toAsyncIterableEffect` captures the current services and returns an async iterable for a TypeScript consumer. `toAsyncIterable` is the direct variant for a source with no service requirements. Iteration failures cross as JavaScript errors, so a foreign consumer must map them back into its declared failure channel; conversion itself does not consume the stream.

```ts
// compiler-gap: D14/D47 A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value.
import { Pull, Stream } from "effect"

function printBatches() {
  using pull = Stream.toPull(Stream.rechunk(Stream.range(30, 34), 2))
  while (true) {
    const batch = Pull.catchDone(() => pull(), () => undefined)
    if (batch === undefined) break
    Effect.log(batch)
  }
}
```

`toPull` returns an Effect value that yields a non-empty array on each call. Normal completion uses `Cause.Done`, handled here with `Pull.catchDone`; other failures propagate. Keep the pull within its Scope. For ordinary element processing, use Stream's consumers or `for (const element of source) { … }`, which pulls a Stream and closes it when the loop exits. `for...of` also consumes EffectScript producers; async iterables use Stream adaptation or a TypeScript consumer.

## Mapping, flattening, and state

```ts
import { Stream } from "effect"

function loadLabel(id: number) {
  Effect.sleep("10 millis")
  return `item-${id}`
}

const labels = Stream.mapEffect(Stream.make(4, 7, 9), (id) => loadLabel(id), { concurrency: 2 })
const upper = Stream.map(labels, (label) => label.toUpperCase())
const signals = Stream.map(labels, () => null)
const words = Stream.flattenIterable(Stream.map(Stream.make("red green", "blue"), (line) => line.split(" ")))
const totals = Stream.mapAccum(Stream.make(4, 7, 9), () => 0, (total, n) => {
  const next = total + n
  return [next, [next]] as const
})
```

`map` is a synchronous value transformation; `mapEffect` accepts EffectScript calls. With concurrency greater than one, `mapEffect` runs multiple callbacks but preserves input order by default; `{ unordered: true }` allows completion order. `flattenIterable` emits each member of each iterable.

`mapAccum` initializes fresh state for each run and returns `[nextState, emittedValues]` per input. The output array can contain zero or more values. Use `mapAccumEffect` when the transition performs Effect work, and `Stream.drain(source)` when work should run but its emitted values should be suppressed.

## Filtering, scanning, and changes

```ts
import { Stream } from "effect"

const readings = Stream.make(2, 2, 5, 5, 2)
const large = Stream.filter(readings, (n) => n >= 5)
const changes = Stream.changes(readings)
const running = Stream.scan(readings, () => 0, (total, n) => total + n)

function inspect() {
  return {
    large: Stream.runCollect(large),     // [5, 5]
    changes: Stream.runCollect(changes), // [2, 5, 2]
    running: Stream.runCollect(running)  // [0, 2, 4, 9, 14, 16]
  }
}
```

`filter` removes elements without changing the survivors. `scan` emits the initial state and each updated state, while `runFold` returns only the final state. `changes` removes consecutive equal elements; it does not remember every distinct value ever seen. `filterEffect`, `scanEffect`, and `changesWithEffect` support Effect work; `changesWith` accepts a custom synchronous equality test.

## Zipping and indexing

```ts
import { Stream } from "effect"

const sizes = Stream.make(5, 8, 13)
const names = Stream.make("small", "large")
const pairs = Stream.zip(sizes, names)
const descriptions = Stream.zipWith(sizes, names, (size, name) => `${name}: ${size}`)
const indexed = Stream.zipWithIndex(names)
const neighbors = Stream.zipWithPreviousAndNext(sizes)

function readPairs() {
  return Stream.runCollect(pairs) // [[5, "small"], [8, "large"]]
}
```

`zip` pairs by position and ends when either side cannot provide another element. `zipWith` computes a value for each pair; `zipWithIndex` adds a zero-based index. `zipWithPrevious`, `zipWithNext`, and `zipWithPreviousAndNext` represent missing neighbors with `Option.none()`.

For sources with different rates, `zipLatest` waits for both sides to produce, then pairs new arrivals with the other side's latest value. `zipLatestWith` applies a combining callback. Timing determines the pairs, so avoid relying on an exact interleaving in real-time examples.

## Cartesian products

```ts
import { Stream } from "effect"

const variants = Stream.cross(Stream.make("linen", "wool"), Stream.make("blue", "gray"))
const names = Stream.crossWith(
  Stream.make("linen", "wool"),
  Stream.make("blue", "gray"),
  (fabric, color) => `${color} ${fabric}`
)
```

`cross` emits every left/right combination. `crossWith` transforms each combination. The right source runs again for each left element, including its effects; an infinite right source prevents progress to the next left element.

## Partitioning and grouping

```ts
// compiler-gap: D14/D47 A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value.
import { Filter, Stream } from "effect"

function partitionReadings() {
  using branches = Stream.partition(
    Stream.range(10, 19),
    Filter.fromPredicate((n: number) => n % 2 === 0),
    { capacity: 2 }
  )
  const [even, odd] = branches
  return Effect.all([
    () => Stream.runCollect(even),
    () => Stream.runCollect(odd)
  ], { concurrency: 2 })
}
```

`partition` runs one source and returns the passing and failing streams in that order. It is an Effect requiring Scope, so keep both consumers within the lifetime of `using`. Consume branches concurrently: a full, unconsumed branch can backpressure the other branch. `partitionEffect` accepts a `Filter.FilterEffect`, whose successes select the passing branch and failures of its Result select the other branch.

```ts
import { Stream } from "effect"

const requests = Stream.make(
  { region: "west", bytes: 10 },
  { region: "east", bytes: 20 },
  { region: "west", bytes: 15 }
)
const groups = Stream.groupByKey(requests, (request) => request.region)
const summaries = Stream.flatMap(groups, ([region, group]) =>
  Stream.fromEffect(() => {
    const bytes = Stream.runFold(group, () => 0, (total, request) => total + request.bytes)
    return { region, bytes }
  }),
  { concurrency: "unbounded" }
)
const batches = Stream.grouped(requests, 2)
const timedBatches = Stream.groupedWithin(requests, 20, "1 second")
```

`groupByKey` produces `[key, groupStream]` pairs. Consume groups concurrently so inactive group consumers do not stall distribution; merged group results have no fixed order. `groupBy` accepts an effectful callback returning `[key, value]`. `grouped` emits arrays of a fixed maximum size, and `groupedWithin` also flushes on elapsed time; both emit a final partial batch.

## Concatenating, merging, and interleaving

```ts
import { Stream } from "effect"

const primary = Stream.make("a1", "a2")
const secondary = Stream.make("b1", "b2", "b3")
const sequential = Stream.concat(primary, secondary)
const flattened = Stream.flatten(Stream.make(primary, secondary))
const expanded = Stream.flatMap(primary, (label) => Stream.make(label, label.toUpperCase()))
const merged = Stream.merge(primary, secondary, { haltStrategy: "both" })
const alternating = Stream.interleave(primary, secondary)
const selected = Stream.interleaveWith(primary, secondary, Stream.make(true, false, false, true, false))
```

`concat` consumes the left source before the right; `flatten` does the same for a stream of streams. `flatMap` builds each inner stream from an element; its default is sequential, while its `concurrency` option runs inner streams together. `switchMap` interrupts the previous inner stream when a new source element arrives, which suits searches where only the newest request matters.

`merge` emits whichever side is ready and preserves each side's order. Its `haltStrategy` accepts `"both"` (the default), `"left"`, `"right"`, or `"either"`; a source failure still fails the merge. Transform each side with `map` before merging when the output needs a common representation.

`interleave` alternates elements and drains the surviving source after the other ends. `interleaveWith` uses a boolean stream: `true` requests left and `false` requests right. It ends when the selector ends or both data streams are exhausted.

## Separators and affixes

```ts
import { Stream } from "effect"

const fields = Stream.make("name", "email", "role")
const separated = Stream.intersperse(fields, ",")
const bracketed = Stream.intersperseAffixes(fields, { start: "[", middle: ",", end: "]" })

function formatFields() {
  return Stream.mkString(bracketed) // "[name,email,role]"
}
```

`intersperse` inserts a separator between adjacent elements. `intersperseAffixes` also emits a prefix and suffix. These are stream elements, so downstream transformations see them too.

## Broadcasting and buffering

```ts
// compiler-gap: D14/D47 A using acquisition must bind its Scope to the block without requiring Symbol.dispose on the resource value.
import { Stream } from "effect"

function shareReadings() {
  using copies = Stream.broadcastN(Stream.range(1, 12), { n: 2, capacity: 3 })
  const [totals, audit] = copies
  return Effect.all([
    () => Stream.runSum(totals),
    () => Stream.runForEach(audit, (n) => {
      Effect.sleep("10 millis")
      Effect.log(`audit ${n}`)
    })
  ], { concurrency: 2 })
}

const buffered = Stream.buffer(Stream.range(1, 12), { capacity: 3 })
```

`broadcastN` starts one source and gives each subscriber the same elements. With bounded suspending buffers, the slowest subscriber limits progress. Keep the subscribers inside the acquired Scope and run their consumers concurrently.

`buffer` lets upstream run ahead of one consumer. `{ capacity: n }` applies backpressure when full; `"unbounded"` removes that limit. With a numeric capacity, `strategy: "sliding"` drops old queued values and `strategy: "dropping"` rejects new ones, trading completeness for throughput.

## Debouncing and throttling

```ts
import { Stream } from "effect"

const bursts = Stream.concat(
  Stream.make("c", "ca", "cat"),
  Stream.fromEffect(() => {
    Effect.sleep("300 millis")
    return "cats"
  })
)
const settled = Stream.debounce(bursts, "100 millis")
const paced = Stream.throttle(Stream.rechunk(Stream.range(1, 10), 1), {
  cost: (batch) => batch.length,
  units: 2,
  duration: "1 second",
  strategy: "shape",
  burst: 1
})
```

`debounce` keeps the latest element during a burst and emits it after the quiet interval. `throttle` uses a token bucket: `units` replenish per `duration`, `cost` charges each internal batch, and `burst` adds bucket capacity. The default `"shape"` strategy waits for tokens; `"enforce"` drops batches that exceed the budget. Rechunk to size one when each element should be charged independently.

## Scheduling delivery

```ts
import { Schedule, Stream } from "effect"

const announcements = Stream.schedule(
  Stream.make("prepare", "deploy", "verify"),
  Schedule.spaced("500 millis")
)

function announce() {
  Stream.runForEach(announcements, (message) => Effect.log(message))
}
```

`schedule` controls when elements are delivered according to a Schedule. It preserves the elements and can also end when the schedule ends. Use `Stream.tick` to generate periodic signals, and `Stream.fromEffectSchedule` to repeat an operation under a schedule.
