# Retrying

Retry transient domain failures when a new attempt has a useful chance of succeeding. A retry runs the supplied program again from its beginning, including its side effects.

[Upstream page](https://effect.website/docs/v4/error-management/retrying)

## Retrying failed computations

```ts
class Busy extends TaggedError("Busy")<{ readonly attempt: number }> {}

function loadWithRetries() {
  let attempts = 0
  return Effect.retry(() => {
    attempts += 1
    if (attempts < 3) throw new Busy({ attempt: attempts })
    return "Snapshot ready"
  }, { times: 3 })
}
```

The initial attempt always runs. `{ times: 3 }` allows three additional attempts, for at most four executions; this example succeeds on its third execution. State declared outside the retry callback, such as `attempts`, persists across attempts, while local variables inside it start fresh.

Failures stop the current attempt before retry begins. Defects and interruption are not retry signals. Retried writes need an application policy that makes repeated execution safe.

## Selecting retryable domain errors

```ts
class RequestRejected extends TaggedError("RequestRejected")<{
  readonly transient: boolean
}> {}

function sendRequest(): Effect<string, RequestRejected, never> {
  throw new RequestRejected({ transient: false })
}

function sendTransient() {
  return Effect.retry(() => sendRequest(), {
    times: 4,
    while: (error) => error.transient
  })
}
```

`while` permits retry while its predicate holds; `until` stops retry when its predicate holds. Both can return a boolean or an Effect, so selection can consult a service when needed. Unselected errors remain typed failures rather than becoming successful results.

## Controlling timing and limits

```ts
import { Schedule } from "effect"

function sendWithBackoff() {
  return Effect.retry(() => sendRequest(), {
    times: 4,
    while: (error) => error.transient,
    schedule: Schedule.exponential("150 millis")
  })
}

function sendThreeMoreTimes() {
  return Effect.retry(() => sendRequest(), Schedule.recurs(3))
}
```

A schedule controls delay and can add its own stopping condition. Combining `times`, predicates, and a schedule stops when the applicable policy says to stop. `Schedule.recurs(3)` permits three retries; use `Effect.repeat` for repetition after success.

## Recovering after retries end

```ts
import { Schedule } from "effect"

function sendOrQueue() {
  return Effect.retryOrElse(
    () => sendRequest(),
    Schedule.recurs(2),
    (error, retries) => {
      Effect.log(`Queued after ${retries} retries; transient=${error.transient}`)
      return "Queued for later"
    }
  )
}
```

`retryOrElse` runs recovery after its schedule is exhausted. The callback receives the last error and the schedule's output, which is a retry count for `Schedule.recurs`. The recovery computation can introduce its own errors and requirements.

## Coming from Effect

Pass `() => sendRequest()` to retry. Passing the result of `sendRequest()` would run the request before the retry policy receives it.
