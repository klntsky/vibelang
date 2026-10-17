# Introduction to Streams

`Stream` describes a sequence that produces values as a consumer asks for them. Use it for large inputs, ongoing events, and sources that need cancellation, resource management, or backpressure. Stream constructors and operators build values; consuming a stream runs an Effect.

## Element, error, and service types

```ts
error FeedUnavailable

service Feed {
  next(): string!FeedUnavailable
}

const messages = Stream.fromEffectRepeat(() => Feed.next())
// Stream.Stream<string, FeedUnavailable, Feed>

function preview() {
  return messages.take(4).runCollect()
}
// Array<string>!FeedUnavailable!Feed
```

`Stream.Stream<A, E, R>` emits elements of type `A`, can fail with `E`, and needs services `R` during consumption. A failure ends that run of the source unless an operator recovers. Constructing `messages` does not look up `Feed` or request a message.

Effect's Stream, Filter, and Cause value builders are trusted library calls. Call them directly; exceptions from those plain calls are defects. Callbacks invoked synchronously must perform no Effect operations and have no expected failures.

## Choosing a streaming use case

```ts
function firstFiveTickets() {
  const tickets = Stream.iterate(100, (n) => n + 1)
  return tickets.take(5).runCollect()
}
// Returns [100, 101, 102, 103, 104].
```

The source has no natural end, but the consumer requests only five elements. The same approach works for log tails, paginated APIs, or incoming messages. For a small collection already in memory, an array and an ordinary loop often suffice.

## Streams and individual effects

```ts
function oneResult() {
  return [12, 24, 36]
}
// Array<number>!!

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
```

A successful function call returns one result, even if that result is an array. A stream emits zero or more elements; an array can be either one element or the source of many elements. Streams can be empty, finite, or infinite.

## Incremental production and backpressure

```ts
function printReadings() {
  const source = Stream.range(20, 25).mapEffect((n) => {
    log(`prepare ${n}`)
    return n
  })
  source.runForEach((n) => {
    sleep("50 millis")
    log(`received ${n}`)
  })
}
```

The consumer controls demand, so production slows when consumption slows. Streams exchange internal batches, so a source can prepare more than one element per pull. Buffering and concurrency introduce explicit read-ahead; backpressure does not mean exactly one element is ever in memory.

```ts
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
```

A `function*` producer is a Stream wherever a Stream is expected, so Stream operators accept it directly. The adapter runs the producer on its own fiber with an unbuffered handoff: it hands over each element only when pulled. `for...of` also consumes an Effect Stream directly, pulling batches and closing the stream when the loop exits. Here `break` closes the source and interrupts the producer fiber, running its cleanup.

## Coming from Effect

- Keep Stream constructors and pipelines. Pass `() => work()` where a constructor expects an Effect.
- `Stream.runCollect(source)` runs at the call and returns an array inside a function.
