# Schedule cookbook

These recipes use ordinary functions as repeatable programs. Each recipe bounds its attempts or checks, and a successful request stops retrying immediately. `Schedule` values and duration calculations work exactly as in Effect.

## Honoring server retry hints

```ts
import { Duration, Schedule } from "effect"

class Throttled extends TaggedError("Throttled")<{
  readonly retryAfter: Duration.Duration
}> {}

function download() {
  let calls = 0
  const policy = pipe(
    Schedule.exponential("500 millis"),
    Schedule.setInputType<Throttled>(),
    Schedule.modifyDelay(({ duration, input }) =>
      Duration.max(duration, input.retryAfter)),
    Schedule.upTo({ times: 3 })
  )
  return Effect.retry(() => {
    calls++
    if (calls < 3) {
      throw new Throttled({ retryAfter: Duration.seconds(2) })
    }
    return "archive ready"
  }, policy)
}
// Effect<string, Throttled, never>
```

Parse the response's retry hint into a duration at the HTTP boundary. The policy compares that duration with the backoff for the same failure, so the first two waits above are two seconds each. Three permitted retries mean at most four requests, and exhausting them propagates the last `Throttled` failure.

## Polling with response-dependent delays

```ts
import { Duration, Schedule } from "effect"

type Status =
  | { readonly _tag: "Pending"; readonly pollAfter: Duration.Duration }
  | { readonly _tag: "Ready"; readonly path: string }

class PollLimit extends TaggedError("PollLimit")<{}> {}

function waitForArchive() {
  let checks = 0
  function check(): Effect<Status, never, never> {
    checks++
    return checks < 3
      ? { _tag: "Pending", pollAfter: Duration.millis(checks * 300) }
      : { _tag: "Ready", path: "/archives/week.csv" }
  }
  const delay = pipe(
    Schedule.recurs(5),
    Schedule.setInputType<Status>(),
    Schedule.modifyDelay(({ input }) =>
      input._tag === "Pending" ? input.pollAfter : Duration.zero)
  )
  const result = Effect.repeat(() => check(), {
    schedule: delay,
    until: (status) => status._tag === "Ready"
  })
  if (result._tag === "Pending") throw new PollLimit()
  return result.path
}
// Effect<string, PollLimit, never>
```

A pending response is a success, so it feeds `repeat` rather than `retry`. This simulation waits 300 milliseconds, then 600 milliseconds, and returns the path on its third check. The count limit permits at most six checks; reaching it with another pending result becomes `PollLimit`.

## Bounding the whole retry operation

```ts
import { Schedule } from "effect"

class Offline extends TaggedError("Offline")<{}> {}

function readWithDeadline() {
  return pipe(
    () => { throw new Offline() },
    Effect.retry(Schedule.upTo(Schedule.exponential("80 millis"), { times: 5 })),
    Effect.timeout("350 millis")
  )
}
```

The outer timeout covers the first attempt, every wait, and every retry. Placing a timeout inside the retried program instead gives each attempt a separate budget. A schedule's elapsed-duration limit is checked only at its steps and cannot interrupt an attempt already in progress; a timeout interrupts the work and waits for its cleanup.

## Switching from rapid to slower retries

```ts
import { Schedule } from "effect"

class Disconnected extends TaggedError("Disconnected")<{}> {}

function reconnect() {
  let attempts = 0
  const phases = Schedule.concat(
    Schedule.upTo(Schedule.spaced("200 millis"), { times: 2 }),
    Schedule.upTo(Schedule.spaced("4 seconds"), { times: 3 })
  )
  return Effect.retry(() => {
    attempts++
    if (attempts < 4) throw new Disconnected()
    return "connected"
  }, phases)
}
```

The first two retries use short waits; later retries use four second waits. This simulation succeeds on attempt four, after entering the slow phase. If every attempt fails, the two phases allow five retries plus the initial attempt.

## Recording retry decisions

```ts
import { Duration, Schedule } from "effect"

class Unavailable extends TaggedError("Unavailable")<{
  readonly status: number
}> {}

function observedRequest() {
  let attempts = 0
  const policy = pipe(
    Schedule.exponential("250 millis"),
    Schedule.setInputType<Unavailable>(),
    Schedule.upTo({ times: 3 }),
    Schedule.tap(({ attempt, duration, input }) => {
      Effect.logWarning("retry planned", {
        retry: attempt,
        status: input.status,
        delayMs: Duration.toMillis(duration)
      })
    })
  )
  return Effect.retry(() => {
    attempts++
    if (attempts < 3) throw new Unavailable({ status: 503 })
    return "accepted"
  }, policy)
}
```

Put `tap` after the limit so it records accepted retries only. The retry number starts at one and describes the next planned attempt, before its wait begins. Telemetry failures also fail the schedule, so handle an export failure inside the callback when it must not stop the request.
