# Using schedules

A schedule decides when work runs again and when it stops. `Schedule` values work in Effeck exactly as in Effect; pass a program to `Effect.repeat`, `Effect.schedule`, or `Effect.retry` to run one. These schedules live in the running process and do not recover jobs missed during downtime.

## Repeating successful computations

```ts
function heartbeat() {
  repeat(
    logInfo("worker available"),
    Schedule.spaced("15 seconds")
  )
}
```

`Effect.repeat` runs the program immediately, then steps the schedule after each success. Here, each completed log is followed by a fifteen second wait. A failure stops the loop, and interruption cancels it.

## Stopping based on a result

```ts
function waitForBuild() {
  let checks = 0
  const status = repeat(() => {
    checks++
    return checks >= 4 ? "ready" : "building"
  }, {
    schedule: Schedule.spaced("200 millis"),
    until: (status) => status === "ready",
    times: 7
  })
  return { status, checks }
}
// { status: string; checks: number }!!
```

The initial check is separate from the seven permitted repetitions, so this loop makes at most eight checks. `until` stops on a matching result; `while` continues while its predicate holds. If the count limit wins, `repeat` returns the last result, so check it before assuming the work is ready.

## Delaying the initial run

```ts
function delayedCleanup() {
  Effect.schedule(
    logInfo("sweeping expired sessions"),
    Schedule.spaced("30 minutes")
  )
}
```

`Effect.schedule` waits for the schedule before the first execution too. This job waits thirty minutes at startup, then thirty minutes after each completed sweep. Use it for calendar jobs and startup delays where an immediate run would be wrong.

## Retrying failed computations

```ts
error Busy

function reserveSlot() {
  let attempts = 0
  return retry(() => {
    attempts++
    if (attempts < 3) throw new Busy()
    return "reserved"
  }, Schedule.recurs(3))
}
// string!Busy!
```

`retry` steps its schedule after an expected failure and stops immediately on success. `recurs(3)` permits three retries after the first attempt, for four attempts at most. Retry does not turn defects or interruption into expected failures.

## Handling failures while repeating

```ts
error SensorOffline

service Sensor {
  read(): number!SensorOffline
}

function sampleContinuously() {
  repeat(() => {
    try {
      logInfo("sample", Sensor.read())
    } catch (error) {
      logWarning("sample unavailable", error._tag)
    }
  }, Schedule.spaced("5 seconds"))
}
```

Handle a recoverable failure inside the repeated program when later samples should still run. A `catch` outside `repeat` recovers after the loop has already stopped. `Effect.repeatOrElse` instead stops repetition and calls a fallback with the failure and an `Option` of the latest schedule metadata; before any schedule step, that option is empty.

## Coming from Effect

Pass `() => sample()` where Effect expects the computation. Schedule constructors and count semantics stay the same; ordinary statements replace the generator body.
