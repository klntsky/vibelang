# Scheduling work with cron

`Cron` and `Schedule.cron` work in VibeLang exactly as in Effect. Use them for work tied to calendar dates, such as a weekday report; use interval schedules when only elapsed time matters.

## Defining calendar execution

```vibe
// run: provide TestClock.layer() { TestClock.setTime(Date.parse("2026-11-02T00:00:00Z")); const worker = fork reportJob(); Console.log("waiting for Monday morning"); TestClock.adjust("3 days"); worker.interrupt() }
import { TestClock } from "effect/testing"

const weekdayReport = Schedule.cron("45 7 * * 1-5", "UTC")

function reportJob() {
  Effect.schedule(Console.log("generating daily report"), weekdayReport)
}
// Output: waiting for Monday morning
// Output: generating daily report
// Output: generating daily report
// Output: generating daily report
```

The five fields are minute, hour, day of month, month, and weekday. This rule runs at 07:45 Monday through Friday; with the optional leading seconds field, `0 45 7 * * 1-5` is equivalent. Fields accept `*`, ranges, comma separated lists, and steps such as `*/10`.

When day of month and weekday both restrict dates, either field can match, so leave day of month as `*` in a weekday rule. `Effect.schedule` waits for the first match, so the report doesn't run immediately at startup.

## Selecting a time zone

```vibe
// run: for (const date of ["2026-10-30T00:00:00Z", "2026-11-02T00:00:00Z"]) { const waits: number[] = []; for (const rule of [localMorning, fixedUtc]) { const step = Schedule.toStep(rule); const result = Effect.result(step(Date.parse(date), undefined)); if (result.isSuccess()) waits.push(result.success[1].toMillis()) }; Console.log(JSON.stringify(waits)) }
const localMorning = Schedule.cron("45 7 * * 1-5", "America/Los_Angeles")
const fixedUtc = Schedule.cron("45 7 * * 1-5", "UTC")
// Output: [53100000,27900000]
// Output: [56700000,27900000]
```

A named zone follows its local calendar, daylight saving changes included; `UTC` keeps one offset all year. Test dates near clock changes, especially times that disappear or occur twice, and pass the zone explicitly instead of depending on the machine's location.

## Validating expressions

```vibe
// run: Console.log(Effect.result(() => validateRule("45 7 * * 1-5", "UTC")).isSuccess()); for (const [expression, zone] of [["not cron", "UTC"], ["* * * * *", "invalid/zone"]]) { const result = Effect.result(() => validateRule(expression, zone)); if (result.isFailure()) Console.log(result.failure._tag) }
function validateRule(expression: string, timeZone: string) {
  return Effect.fromResult(Cron.parse(expression, timeZone))
}
// Effect<Cron.Cron, Cron.CronParseError>
// Output: true
// Output: CronParseError
// Output: CronParseError
```

`Cron.parse` returns a `Result`, so parsing alone throws no typed failure; `Effect.fromResult` puts invalid expressions and invalid zones in the error channel. Validate configuration before starting any work, and report the configuration error at the application boundary.

## Previewing future runs

```vibe
// run: Console.log(JSON.stringify(preview()))
function preview() {
  const rule = Effect.fromResult(Cron.parse("45 7 * * 1-5", "UTC"))
  let after = new Date("2026-11-02T00:00:00Z")
  const dates: string[] = []
  for (let i = 0; i < 3; i++) {
    after = Cron.next(rule, after)
    dates.push(after.toISOString())
  }
  return dates
}
// Output: ["2026-11-02T07:45:00.000Z","2026-11-03T07:45:00.000Z","2026-11-04T07:45:00.000Z"]
```

`Cron.next` finds the next matching instant after the given date; explicit dates make a preview reproducible. `Cron.sequence` iterates over future matches, and `Cron.match` tests an existing date against a rule.

## Starting and stopping a job

```vibe
// run: provide TestClock.layer() { TestClock.setTime(Date.parse("2026-11-02T00:09:45Z")); const app = fork runApplication(); TestClock.adjust("31 seconds"); join app; Console.log("application stopped"); TestClock.adjust("10 minutes") }
function runApplication() {
  const rule = Effect.fromResult(Cron.parse("*/10 * * * *", "UTC"))
  const reports = fork(() => Effect.schedule(
    Console.log("report tick"),
    Schedule.cron(rule)
  ))

  sleep("30 seconds") // Stand-in for the application's active lifetime.
  reports.interrupt()
}
// Output: report tick
// Output: application stopped
```

Keep the job's fiber alive for the part of the application that needs it. `Fiber.interrupt` waits for cleanup, and block ownership also stops this child when `runApplication` exits. A failure stops the schedule unless the action handles it or retries internally.

These cron jobs exist only in memory. For a job that must survive restarts or recover missed dates, use an external scheduler or durable pending-work records.
