# Creating Sinks

Choose a Sink for the result you need and run it with `Stream.run`. Sinks are reusable: initializer state is fresh on each run, so runs share no mutable state. This page uses [trusted Sink builders](059-introduction.md) and writes effectful callbacks as ordinary VibeLang functions.

## Basic consumers and endpoint values

```vibe
// run: Console.log(JSON.stringify([endpoints().first.getOrElse(() => ""), endpoints().last.getOrElse(() => ""), endpoints().prefix]))
function endpoints() {
  const source = Stream.make("opening", "middle", "closing")
  return {
    first: source.run(Sink.head<string>()),
    last: source.run(Sink.last<string>()),
    prefix: source.run(Sink.take<string>(2))
  }
}
// first: Some("opening"), last: Some("closing")
// prefix: ["opening", "middle"]
// Output: ["opening","closing",["opening","middle"]]
```

`head` stops after the first element; `last` reads to the end; both return `Option.none()` on empty input. `take(n)` returns up to `n` elements, completing early at the limit and keeping any unused suffix of its current input batch as leftovers.

## Counting, summing, draining, and timing

```vibe
// run: (() => { const result = summarize(); Console.log(JSON.stringify([result.count, result.sum, Duration.isDuration(result.duration)])); drainObserved() })()
function summarize() {
  const source = Stream.make(6, 10, 14)
  return {
    count: source.run(Sink.count), // 3
    sum: source.run(Sink.sum),     // 30
    duration: source.run(Sink.timed)
  }
}

function drainObserved() {
  const source = Stream.make("ready", "done").tap((s) => Console.log(s))
  source.run(Sink.drain)
}
// Output: [3,30,true]
// Output: ready
// Output: done
```

These sinks consume the whole source. `drain` discards elements but still runs source effects; `timed` also drains and returns a `Duration.Duration`. `Sink.withDuration(otherSink)` returns `[result, duration]`: the other sink's result and the elapsed time. Each field in `summarize` is a separate consumption.

## Per-element work

```vibe
// run: writeRecords()
const print = Sink.forEach((line: string) => {
  Console.log(`record: ${line}`)
  sleep("10 millis")
})

function writeRecords() {
  Stream.make("created", "approved").run(print)
}
// Output: record: created
// Output: record: approved
```

`forEach` runs work in sequence and returns `void`; its callback's expected failures and services become the sink's `E` and `R`. `forEachWhile` stops when its effectful callback returns `false`; `forEachArray` works on internal batches.

## Successful and failed sinks

```vibe
// run: (() => { Console.log(immediateResult()); try { rejectInput() } catch (error) { Console.log(error._tag) } })()
error Rejected

function immediateResult() {
  return Stream.make(40, 50).run(Sink.succeed("already complete"))
}

function rejectInput() {
  return Stream.make(40, 50).run(Sink.fail(new Rejected()))
}
// never!Rejected!
// Output: already complete
// Output: Rejected
```

`succeed` and `fail` finish without requesting source elements. They succeed or fail when the sink runs, not at construction. `Sink.fromEffect(() => work())` makes the same kind of immediate sink from a VibeLang program.

## Collecting by size or condition

```vibe
// run: Console.log(JSON.stringify(collectSamples()))
function collectSamples() {
  const source = Stream.make(8, 5, -1, 13)
  return {
    all: source.run(Sink.collect<number>()),
    firstTwo: source.run(Sink.take<number>(2)),
    positivePrefix: source.run(Sink.takeWhile((n: number) => n > 0))
  }
}
// all: [8, 5, -1, 13], firstTwo: [8, 5], positivePrefix: [8, 5]
// Output: {"all":[8,5,-1,13],"firstTwo":[8,5],"positivePrefix":[8,5]}
```

`collect` gathers every input into an array. `takeWhile` stops at the first rejected element and leaves it in the batch remainder; it does not skip it and keep searching. `takeUntil` includes the matching element, then stops. `takeWhileEffect` and `takeUntilEffect` accept effectful predicates.

## Uniqueness and key limits

```vibe
// run: Console.log(JSON.stringify(Array.from(collectUnique())))
const unique = Sink.reduce(() => new Set<string>(), (set, value: string) => set.add(value))
const threeUnique = Sink.reduceWhile(
  () => new Set<string>(),
  (set) => set.size < 3,
  (set, value: string) => set.add(value)
)

function collectUnique() {
  return Stream.make("oak", "oak", "ash", "elm", "pine").run(threeUnique)
}
// Set containing "oak", "ash", "elm".
// Output: ["oak","ash","elm"]
```

Use a native Set when JavaScript value equality fits. A size limit counts unique values, so duplicates take no extra slot.

```vibe
// run: Console.log(JSON.stringify(Array.from(summarizeStations())))
type Reading = { station: string; count: number }

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
  return Stream.make(
    { station: "hill", count: 4 },
    { station: "hill", count: 3 },
    { station: "lake", count: 8 },
    { station: "hill", count: 9 }
  ).run(twoStations)
}
// Map with "hill" -> 7 and "lake" -> 8.
// Output: [["hill",7],["lake",8]]
```

The reducer defines both the grouping key and the merge rule. The limited sink stops at two keys and leaves later input unread, even updates for existing keys: it limits the consumed prefix rather than scanning all input while rejecting new keys.

## Folding with termination and count limits

```vibe
// run: Console.log(JSON.stringify(Effect.provide(foldSamples(), Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))]))))
const total = Sink.reduce(() => 0, (sum, n: number) => sum + n)
const budget = Sink.fold(
  () => 0,
  (sum) => sum < 20,
  (sum, n: number) => {
    log(`adding ${n}`)
    return sum + n
  }
)
const firstThree = Sink.foldUntil(() => 0, 3, (sum, n: number) => sum + n)

function foldSamples() {
  const source = Stream.make(7, 9, 12, 15)
  return {
    all: source.run(total),       // 43
    budget: source.run(budget),   // 28
    first: source.run(firstThree) // 28
  }
}
// Output: adding 7
// Output: adding 9
// Output: adding 12
// Output: {"all":43,"budget":28,"first":28}
```

`reduce` takes a pure reducer and consumes to the end; `reduceEffect` supports Effect work. `fold` takes an effectful step and checks its continuation predicate after each consumed element, so the element that reaches or crosses the limit is included. `foldUntil` stops after a maximum number of inputs or at source completion.

## Folding by weight

```vibe
// run: Console.log(JSON.stringify(firstBatch()))
type Packet = { bytes: number }
type Batch = { packets: ReadonlyArray<Packet>; bytes: number }

const weighted = Sink.fold<Batch, Packet>(
  () => ({ packets: [], bytes: 0 }),
  (batch) => batch.bytes < 100,
  (batch, packet) => ({
    packets: [...batch.packets, packet],
    bytes: batch.bytes + packet.bytes
  })
)
const packets = weighted.map((batch) => batch.packets)

function firstBatch() {
  return Stream.make({ bytes: 60 }, { bytes: 55 }, { bytes: 10 }).run(packets)
}
// Returns the 60-byte and 55-byte packets.
// Output: [{"bytes":60},{"bytes":55}]
```

The state tracks total cost, not element count. It stops after reaching the threshold, so the final element can exceed the target: this is not strict maximum-size packing. `Stream.transduce(source, packets)` repeats the sink to emit successive batches, feeding each run's leftovers into the next. At end of input, a sink that consumes nothing can produce an empty final result; batch consumers must handle it.
