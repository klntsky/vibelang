# Using schedules

A schedule decides when work runs again and when it stops. `Schedule` values work in VibeLang exactly as in Effect; pass a program to `Effect.repeat`, `Effect.schedule`, or `Effect.retry` to run one. These schedules live in the running process and do not recover jobs missed during downtime.

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

`Effect.repeat` runs the program immediately, then steps the schedule after each success. Here, each completed log is followed by a fifteen second wait. A failure stops the loop, and interruption cancels it.

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
// { status: string; checks: number }!!
// Output: {"status":"ready","checks":4}
```

The initial check is separate from the seven permitted repetitions, so this loop makes at most eight checks. `until` stops on a matching result; `while` continues while its predicate holds. If the count limit wins, `repeat` returns the last result, so check it before assuming the work is ready.

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

`Effect.schedule` waits for the schedule before the first execution too. This job waits thirty minutes at startup, then thirty minutes after each completed sweep. Use it for calendar jobs and startup delays where an immediate run would be wrong.

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
// string!Busy!
// Output: reserved
```

`retry` steps its schedule after an expected failure and stops immediately on success. `recurs(3)` permits three retries after the first attempt, for four attempts at most. Retry does not turn defects or interruption into expected failures.

## Handling failures while repeating

```vibe
// run: let reads = 0; provide Layer.mergeAll(TestClock.layer(), Layer.succeed(Sensor, { read: () => { reads++; if (reads !== 2) throw new SensorOffline(); return 22 } })) { const worker = fork sampleContinuously(); TestClock.adjust("11 seconds"); worker.interrupt() }
error SensorOffline

service Sensor {
  read(): number!SensorOffline
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

Handle a recoverable failure inside the repeated program when later samples should still run. A `catch` outside `repeat` recovers after the loop has already stopped. `Effect.repeatOrElse` instead stops repetition and calls a fallback with the failure and an `Option` of the latest schedule metadata; before any schedule step, that option is empty.

## Coming from Effect

Pass `() => sample()` where Effect expects the computation. Schedule constructors and count semantics stay the same; ordinary statements replace the generator body.
