# Using schedules

A schedule decides when work runs again and when it stops. `Schedule` values work in VibeLang exactly as in Effect; to run one, pass a program to `Effect.repeat`, `Effect.schedule`, or `Effect.retry`. Schedules live in the running process and don't recover jobs missed during downtime.

## Repeating successful computations

```vibe
// run: provide TestClock.layer() { const worker = fork heartbeat(); TestClock.adjust("46 seconds"); worker.interrupt() }
import { TestClock } from "effect/testing"

function heartbeat() {
  repeat(
    Console.log("worker available"),
    Schedule.spaced("15 seconds")
  )
}
// Output: worker available
// Output: worker available
// Output: worker available
// Output: worker available
```

`Effect.repeat` runs the program immediately, then steps the schedule after each success: here, a fifteen second wait after each completed log. A failure stops the loop; interruption cancels it.

## Stopping based on a result

```vibe
// run: Console.log(JSON.stringify(waitForBuild()))
function waitForBuild() {
  let checks = 0
  const status = repeat({
    schedule: Schedule.spaced("200 millis"),
    until: (status) => status === "ready",
    times: 7
  }) {
    checks++
    return checks >= 4 ? "ready" : "building"
  }
  return { status, checks }
}
// Effect<{ status: string; checks: number }>
// Output: {"status":"ready","checks":4}
```

The initial check is separate from the seven permitted repetitions, so the loop makes at most eight checks. `until` stops on a matching result; `while` continues while its predicate holds. If the count limit wins, `repeat` returns the last result, so check it before assuming the work is ready.

## Delaying the initial run

```vibe
// run: provide TestClock.layer() { const worker = fork delayedCleanup(); Console.log("before first sweep"); TestClock.adjust("60 minutes"); worker.interrupt() }
function delayedCleanup() {
  Effect.schedule(
    Console.log("sweeping expired sessions"),
    Schedule.spaced("30 minutes")
  )
}
// Output: before first sweep
// Output: sweeping expired sessions
// Output: sweeping expired sessions
```

`Effect.schedule` also waits for the schedule before the first run: this job waits thirty minutes at startup, then thirty minutes after each completed sweep. Use it for calendar jobs and startup delays where an immediate run is wrong.

## Retrying failed computations

```vibe
// run: Console.log(reserveSlot())
error Busy

function reserveSlot() {
  let attempts = 0
  return retry(Schedule.recurs(3)) {
    attempts++
    if (attempts < 3) throw new Busy()
    return "reserved"
  }
}
// Effect<string, Busy>
// Output: reserved
```

`retry` steps its schedule after an expected failure and stops immediately on success. `recurs(3)` permits three retries after the first attempt: four attempts at most. Retry doesn't turn defects or interruption into expected failures.

## Handling failures while repeating

```vibe
// run: let reads = 0; provide Layer.mergeAll(TestClock.layer(), Layer.succeed(Sensor, { read: () => { reads++; if (reads !== 2) throw new SensorOffline(); return 22 } })) { const worker = fork sampleContinuously(); TestClock.adjust("11 seconds"); worker.interrupt() }
error SensorOffline

service Sensor {
  read(): Effect<number, SensorOffline>
}

function sampleContinuously() {
  repeat(Schedule.spaced("5 seconds")) {
    try {
      Console.log("sample", Sensor.read())
    } catch (error) {
      Console.log("sample unavailable", error._tag)
    }
  }
}
// Output: sample unavailable SensorOffline
// Output: sample 22
// Output: sample unavailable SensorOffline
```

When later samples should still run, handle a recoverable failure inside the repeated program; a `catch` outside `repeat` recovers after the loop has stopped. `Effect.repeatOrElse` instead stops repetition and calls a fallback with the failure and an `Option` of the latest schedule metadata, empty before any schedule step.

## Coming from Effect

Pass `() => sample()` where Effect expects the computation. Schedule constructors and count semantics are unchanged; ordinary statements replace the generator body.
