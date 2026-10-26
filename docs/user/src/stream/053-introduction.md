# Introduction to Streams

A `Stream` produces values on demand. Use it for large inputs, ongoing events, and sources that need cancellation, resource management, or backpressure. Constructors and operators build values; consuming a stream runs an Effect.

## Element, error, and service types

```vibe
// run: provide Feed = { next: () => "message" } { Console.log(JSON.stringify(preview())) }
error FeedUnavailable

service Feed {
  next(): Effect<string, FeedUnavailable>
}

const messages = Stream.fromEffectRepeat(() => Feed.next())
// Stream.Stream<string, FeedUnavailable, Feed>

function preview() {
  return messages.take(4).runCollect()
}
// Effect<Array<string>, FeedUnavailable, Feed>
// Output: ["message","message","message","message"]
```

`Stream.Stream<A, E, R>` emits `A`, can fail with `E`, and needs services `R` while consumed. A failure ends that run of the source unless an operator recovers. Defining `messages` neither looks up `Feed` nor requests a message.

Effect's Stream, Filter, and Cause value builders are trusted library calls: call them directly; their exceptions are defects. Synchronously invoked callbacks must perform no Effect operations and have no expected failures.

## Choosing a streaming use case

```vibe
// run: Console.log(JSON.stringify(firstFiveTickets()))
function firstFiveTickets() {
  const tickets = Stream.iterate(100, (n) => n + 1)
  return tickets.take(5).runCollect()
}
// Returns [100, 101, 102, 103, 104].
// Output: [100,101,102,103,104]
```

The source never ends; the consumer takes only five elements. Log tails, paginated APIs, and incoming messages work the same way. For a small in-memory collection, an array and a loop often suffice.

## Streams and individual effects

```vibe
// run: Console.log(JSON.stringify([oneResult(), noReadings.runCollect(), oneReading.runCollect(), ...compare()]))
function oneResult() {
  return [12, 24, 36]
}
// Effect<Array<number>>

const noReadings = Stream.empty
const oneReading = Stream.succeed(12)
const readings = Stream.fromArray([12, 24, 36])
const oneBatch = Stream.succeed([12, 24, 36])

function compare() {
  return [
    readings.runCollect(), // [12, 24, 36]
    oneBatch.runCollect()  // [[12, 24, 36]]
  ]
}
// Output: [[12,24,36],[],[12],[12,24,36],[[12,24,36]]]
```

A successful call returns one result, even an array. A stream emits zero or more elements and can be empty, finite, or infinite; an array can be one element or the source of many.

## Incremental production and backpressure

```vibe
// run: printReadings()
function printReadings() {
  const source = Stream.range(20, 25).mapEffect((n) => {
    Console.log(`prepare ${n}`)
    return n
  })
  source.runForEach((n) => {
    sleep("50 millis")
    Console.log(`received ${n}`)
  })
}
// Output: prepare 20
// Output: received 20
// Output: prepare 21
// Output: received 21
// Output: prepare 22
// Output: received 22
// Output: prepare 23
// Output: received 23
// Output: prepare 24
// Output: received 24
// Output: prepare 25
// Output: received 25
```

The consumer controls demand, so production slows when consumption does. Streams exchange internal batches, so a source can prepare several elements per pull, and buffering and concurrency add explicit read-ahead: backpressure does not mean one element at a time in memory.

```vibe
// run: Effect.provide(printFirstReading(), Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))]))
function* readings() {
  for (let n = 1; n <= 3; n++) {
    sleep("50 millis")
    yield n
  }
}

function printFirstReading() {
  const source = readings().map((n) => n * 10)
  for (const n of source) {
    log(n) // 10
    break
  }
}
// Output: 10
```

A `function*` producer is a Stream wherever one is expected, so Stream operators accept it. The adapter runs it on its own fiber with an unbuffered handoff, passing each element only when pulled. `for...of` also consumes an Effect Stream, pulling batches and closing it when the loop exits. Here `break` closes the source and interrupts the producer fiber, running its cleanup.

## Coming from Effect

- Keep Stream constructors and pipelines; pass `() => work()` where a constructor expects an Effect.
- In a function, `Stream.runCollect(source)` runs at the call and returns an array.
