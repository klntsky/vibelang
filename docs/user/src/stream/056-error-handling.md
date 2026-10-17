# Error Handling in Streams

Stream recovery replaces a failed source with another source. Use stream operators to control recovery during consumption, and ordinary `try`/`catch` when the whole consuming function should handle a typed failure. Elements emitted before a failure are not rolled back.

## Recovering typed failures and defects

```ts
error Disconnected

const primary = Stream.make("live-a", "live-b").concat(Stream.fail(new Disconnected()))
const recovered = Stream.catch(primary, () => Stream.succeed("cached"))

function readRecovered() {
  return recovered.runCollect() // ["live-a", "live-b", "cached"]
}

const tagged = Stream.catch(
  primary.map((value) => Result.succeed(value)),
  () => Stream.make("cached").map((value) => Result.fail(value))
)
```

`Stream.catch` receives a typed failure and returns the replacement Stream. Recovery abandons the rest of the failed source; it does not resume after its failing step. Tagging each source with `Result` is useful when downstream code needs to distinguish live data from fallback data.

```ts
const boundary = Stream.die(new Error("decoder invariant failed")).catchDefect(() => Stream.succeed("unreadable"))
```

`catchDefect` handles defects at a deliberate boundary. `catchCause` receives the full Cause, including failure, defect, and interruption reasons; use it only when that full information is needed. Ordinary Effeck `catch` handles typed failures only, and propagates a mixed cause containing defects or interruption.

## Selective recovery

```ts
error NotReady
error AccessDenied

function recoverKnown(source: Stream.Stream<string, NotReady | AccessDenied>) {
  return source.catchTag("NotReady", () => Stream.succeed("pending"))
}

const selected = Stream.fail("busy" as "busy" | "forbidden").catchFilter(Filter.fromPredicate((error: "busy" | "forbidden") => error === "busy"), () => Stream.succeed("try later"))

const defectOnly = Stream.die("corrupt frame").catchCauseFilter(Cause.findDie, () => Stream.empty)
```

`catchTag` removes the handled tagged error from the output failure type. `catchFilter` handles values selected by a Filter and preserves failures it rejects. `catchCauseFilter` selects from the full Cause; `Cause.findDie` selects a defect reason rather than treating every failure as recoverable.

## Cleanup after failure

```ts
function consumeChecked() {
  const source = Stream.succeed("accepted")
    .concat(Stream.fail("read failed"))
    .onError(() => log("input failed; closing reader"))
  try {
    return source.runCollect()
  } finally {
    log("consumer finished")
  }
}
```

`onError` runs its callback on an unsuccessful stream exit and preserves the failure if that callback succeeds. It is useful for diagnostics tied to a source. Use `Stream.ensuring` for all exits or `finally` for cleanup belonging to the consuming function; cleanup failure is never silently discarded.

## Retrying a source

```ts
error Offline

service class Sensor {
  read(): number!Offline
}

const readings = Stream.fromEffect(() => Sensor.read()).retry(Schedule.recurs(3))
```

`retry` starts the wrapped source again after a typed failure, according to its schedule. Retrying a multi-element source can repeat elements already delivered. Retry the smallest source that represents the intended operation, and make downstream side effects safe to repeat. An exponential schedule adds backoff; a finite recurrence schedule bounds the number of retries.

## Narrowing error types

```ts
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
error Idle

const endWhenIdle = Stream.never.timeout("3 seconds")
const failWhenIdle = Stream.never.timeoutOrElse({
  duration: "3 seconds",
  orElse: () => Stream.fail(new Idle())
})
const defectWhenIdle = Stream.never.timeoutOrElse({
  duration: "3 seconds",
  orElse: () => Stream.failCause(Cause.die("idle invariant"))
})
const fallbackWhenIdle = Stream.never.timeoutOrElse({
  duration: "3 seconds",
  orElse: () => Stream.make("backup-a", "backup-b")
})
```

`timeout` completes normally when an upstream pull waits too long. `timeoutOrElse` switches to the Stream returned by `orElse`, so that Stream can fail, defect, or emit fallback data. The timeout applies to each upstream pull, not to total consumption time, and does not time the replacement stream.
