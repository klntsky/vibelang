# Error Handling in Streams

Stream recovery replaces a failed source with another source. Use stream operators to control recovery during consumption, and ordinary `try`/`catch` when the whole consuming function should handle a typed failure. Elements emitted before a failure are not rolled back.

## Recovering typed failures and defects

```ts
import { Result, Stream } from "effect"

class Disconnected extends TaggedError("Disconnected")<{}> {}

const primary = Stream.concat(Stream.make("live-a", "live-b"), Stream.fail(new Disconnected({})))
const recovered = Stream.catch(primary, () => Stream.succeed("cached"))

function readRecovered() {
  return Stream.runCollect(recovered) // ["live-a", "live-b", "cached"]
}

const tagged = Stream.catch(
  Stream.map(primary, (value) => Result.succeed(value)),
  () => Stream.map(Stream.make("cached"), (value) => Result.fail(value))
)
```

`Stream.catch` receives a typed failure and returns the replacement Stream. Recovery abandons the rest of the failed source; it does not resume after its failing step. Tagging each source with `Result` is useful when downstream code needs to distinguish live data from fallback data.

```ts
import { Stream } from "effect"

const boundary = Stream.catchDefect(
  Stream.die(new Error("decoder invariant failed")),
  () => Stream.succeed("unreadable")
)
```

`catchDefect` handles defects at a deliberate boundary. `catchCause` receives the full Cause, including failure, defect, and interruption reasons; use it only when that full information is needed. Ordinary EffectScript `catch` handles typed failures only, and propagates a mixed cause containing defects or interruption.

## Selective recovery

```ts
import { Cause, Filter, Stream } from "effect"

class NotReady extends TaggedError("NotReady")<{}> {}
class AccessDenied extends TaggedError("AccessDenied")<{}> {}

function recoverKnown(source: Stream.Stream<string, NotReady | AccessDenied>) {
  return Stream.catchTag(source, "NotReady", () => Stream.succeed("pending"))
}

const selected = Stream.catchFilter(
  Stream.fail("busy" as "busy" | "forbidden"),
  Filter.fromPredicate((error: "busy" | "forbidden") => error === "busy"),
  () => Stream.succeed("try later")
)

const defectOnly = Stream.catchCauseFilter(
  Stream.die("corrupt frame"),
  Cause.findDie,
  () => Stream.empty
)
```

`catchTag` removes the handled tagged error from the output failure type. `catchFilter` handles values selected by a Filter and preserves failures it rejects. `catchCauseFilter` selects from the full Cause; `Cause.findDie` selects a defect reason rather than treating every failure as recoverable.

## Cleanup after failure

```ts
import { Stream } from "effect"

function consumeChecked() {
  const source = pipe(
    Stream.concat(Stream.succeed("accepted"), Stream.fail("read failed")),
    Stream.onError(() => Effect.log("input failed; closing reader"))
  )
  try {
    return Stream.runCollect(source)
  } finally {
    Effect.log("consumer finished")
  }
}
```

`onError` runs its callback on an unsuccessful stream exit and preserves the failure if that callback succeeds. It is useful for diagnostics tied to a source. Use `Stream.ensuring` for all exits or `finally` for cleanup belonging to the consuming function; cleanup failure is never silently discarded.

## Retrying a source

```ts
import { Schedule, Stream } from "effect"

class Offline extends TaggedError("Offline")<{}> {}

abstract class Sensor {
  abstract read(): Effect<number, Offline, never>
}

const readings = Stream.retry(
  Stream.fromEffect(() => Sensor.read()),
  Schedule.recurs(3)
)
```

`retry` starts the wrapped source again after a typed failure, according to its schedule. Retrying a multi-element source can repeat elements already delivered. Retry the smallest source that represents the intended operation, and make downstream side effects safe to repeat. An exponential schedule adds backoff; a finite recurrence schedule bounds the number of retries.

## Narrowing error types

```ts
import { Stream } from "effect"

function keepSyntaxErrors(source: Stream.Stream<string, Error>) {
  return Stream.catch(source, (error) => {
    if (error instanceof SyntaxError) return Stream.fail(error)
    return Stream.die(error)
  })
}
// The returned Stream has E = SyntaxError.
```

Refail the errors callers are expected to handle and turn impossible failures into defects. `Stream.orDie(source)` makes every typed failure a defect. These operators change the failure channel; they do not make a faulty source succeed.

## Timeout failures and replacement sources

```ts
import { Cause, Stream } from "effect"

class Idle extends TaggedError("Idle")<{}> {}

const endWhenIdle = Stream.timeout(Stream.never, "3 seconds")
const failWhenIdle = Stream.timeoutOrElse(Stream.never, {
  duration: "3 seconds",
  orElse: () => Stream.fail(new Idle({}))
})
const defectWhenIdle = Stream.timeoutOrElse(Stream.never, {
  duration: "3 seconds",
  orElse: () => Stream.failCause(Cause.die("idle invariant"))
})
const fallbackWhenIdle = Stream.timeoutOrElse(Stream.never, {
  duration: "3 seconds",
  orElse: () => Stream.make("backup-a", "backup-b")
})
```

`timeout` completes normally when an upstream pull waits too long. `timeoutOrElse` switches to the Stream returned by `orElse`, so that Stream can fail, defect, or emit fallback data. The timeout applies to each upstream pull, not to total consumption time, and does not time the replacement stream.
