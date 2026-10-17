# Creating Streams

Build a Stream from values, effectful work, or an existing source. Construction stays lazy with respect to stream execution: callbacks that perform Effect work run when the stream is consumed. The [introduction](053-introduction.md) explains trusted Stream builders and how producers work with Streams.

## Finite and empty streams

```ts
const labels = Stream.make("queued", "running", "complete")
const absent = Stream.empty
const signal = Stream.succeed(void 0)
const pages = Stream.range(4, 7)
const powers = Stream.take(Stream.iterate(2, (n) => n * 2), 4)

function readPowers() {
  return Stream.runCollect(powers) // [2, 4, 8, 16]
}
```

`make` accepts individual elements and `empty` emits none. `range` includes both endpoints. `iterate` emits the initial value, then repeatedly applies its pure callback; limit an infinite source before collecting it.

## Successes and failures

```ts
class SourceClosed extends TaggedError("SourceClosed")<{}> {}

const ready = Stream.succeed("ready")
const closed = Stream.fail(new SourceClosed())

function readClosed() {
  try {
    return Stream.runCollect(closed)
  } catch (error) {
    return ["source unavailable"]
  }
}
```

`succeed` emits once and completes. `fail` builds a failing source; it does not throw at construction. A `try` around consumption handles its typed failure.

## Arrays and iterables

```ts
const fromArray = Stream.fromArray([11, 22])
const fromBatches = Stream.fromArrays([11, 22], [33])
const fromSet = Stream.fromIterable(new Set(["east", "west"]))

abstract class Catalog {
  abstract codes(): Effect<ReadonlyArray<string>, never, never>
}

const codes = Stream.fromIterableEffect(() => Catalog.codes())
```

`fromArray` and `fromIterable` emit collection members. `fromArrays` preserves the supplied batch boundaries internally. `fromIterableEffect` first runs the program to obtain an iterable, then emits its members; service requirements belong to consumption.

## Async iterables

```ts
class IteratorFailed extends TaggedError("IteratorFailed")<{
  readonly cause: unknown
}> {}

function adaptRows(rows: AsyncIterable<string>) {
  return Stream.fromAsyncIterable(rows, (cause) => new IteratorFailed({ cause }))
}
```

Pass an async iterable obtained from a TypeScript adapter or a service. The error mapper turns iteration exceptions and rejections into the stream's failure type. Effeck has no `async` generator syntax or `for await`; use Stream consumers after adaptation.

## Effects and callbacks

```ts
const timestamp = Stream.fromEffect(() => Clock.currentTimeMillis())

const notices = Stream.callback<string>((queue) => {
  Queue.offer(queue, "connected")
  Queue.offer(queue, "ready")
  Queue.end(queue)
})

function readNotices() {
  return Stream.runCollect(notices) // ["connected", "ready"]
}
```

`fromEffect` emits one success value. `callback` gives its registration program a queue: `Queue.offer` emits an element, `Queue.end` completes normally, and `Queue.fail` terminates with a typed error. Its optional second argument accepts `bufferSize` and a `strategy` of `"suspend"`, `"dropping"`, or `"sliding"`.

For a foreign event API, put the raw callback registration in a TypeScript adapter and register unsubscription with `Effect.acquireRelease`. The registration callback's Scope remains open for the stream lifetime. Platform callbacks cannot call Effeck programs directly as synchronous JavaScript callbacks.

## Scoped sources

```ts
function openSession() {
  Effect.log("open session")
  return Effect.acquireRelease(
    () => ({ name: "metrics" }),
    () => Effect.log("close session")
  )
}

const sessionNames = Stream.scoped(
  Stream.map(Stream.fromEffect(() => openSession()), (session) => session.name)
)
```

`Stream.scoped` takes a Stream, and supplies its Scope during consumption. `Effect.acquireRelease` registers release in that Scope. Place all operations that use the acquired resource inside the scoped stream; see [Resourceful Streams](058-resourceful-streams.md).

## Repeating values, streams, and effects

```ts
const pings = Stream.forever(Stream.succeed("ping"))
const rounds = Stream.repeat(Stream.make("start", "finish"), Schedule.recurs(2))
const times = Stream.fromEffectRepeat(() => Clock.currentTimeMillis())

function countdown() {
  let remaining = 3
  const source = Stream.fromEffectRepeat(() => {
    if (remaining === 0) Cause.done()
    return remaining--
  })
  return Stream.runCollect(source) // [3, 2, 1]
}
```

`forever` restarts its stream indefinitely. `repeat` runs the original stream once and lets the schedule control additional runs; `rounds` emits three rounds. `fromEffectRepeat` reruns its program for each element and treats `Cause.Done` as normal completion; `Cause.done()` runs an Effect that signals completion.

Keep mutable per-run state inside a function, as in `countdown`, so separate consumptions do not share it. `Stream.take` is another way to bound repetition.

## Periodic elements

```ts
const samples = Stream.mapEffect(Stream.tick("250 millis"), () => Clock.currentTimeMillis())

function collectSamples() {
  return Stream.runCollect(Stream.take(samples, 4))
}
```

`tick` emits `void` on an interval. Map each signal to the value you need. Backpressure still applies; a tick stream is not a guarantee that slow work completes at that frequency.

## Unfolding state and paginating responses

```ts
const halving = Stream.unfold(32, (n) => {
  if (n < 2) return undefined
  return [n, n / 2] as const
})

abstract class Directory {
  abstract page(cursor: number): Effect<{
    readonly names: ReadonlyArray<string>
    readonly next: number | undefined
  }, never, never>
}

const names = Stream.paginate(0, (cursor) => {
  const page = Directory.page(cursor)
  return [page.names, Option.fromUndefinedOr(page.next)] as const
})
```

`unfold` returns either `[element, nextState]` or `undefined` to stop without emitting. Its callback expects an Effect, so ordinary Effeck statements can perform work between steps. `paginate` instead returns `[batch, nextStateOption]`; it emits the whole batch, including the final page whose next state is `None`.

## Queues and broadcasts

```ts
function queuedNumbers() {
  const queue = Queue.unbounded<number, Cause.Done>()
  Queue.offer(queue, 8)
  Queue.offer(queue, 13)
  Queue.end(queue)
  return Stream.runCollect(Stream.fromQueue(queue)) // [8, 13]
}

function fromBroadcast(bus: PubSub.PubSub<string>) {
  return Stream.fromPubSub(bus)
}
```

`fromQueue` removes items from a shared queue, so competing consumers divide the work. `fromPubSub` creates a subscription during consumption, so subscribers each receive published elements. A plain PubSub source stays live until the bus ends or the consumer stops; publishing before subscription does not guarantee delivery without replay configuration.

## Schedules as sources

```ts
const policy = pipe(Schedule.spaced("400 millis"), Schedule.upTo({ times: 3 }))
const pulses = Stream.fromSchedule(policy)

function readPulses() {
  return Stream.runCollect(pulses) // [0, 1, 2]
}
```

`fromSchedule` emits schedule outputs until the schedule stops. `Stream.fromEffectSchedule(() => work(), policy)` instead emits repeated program results under a schedule. Both preserve the schedule's timing and service requirements.
