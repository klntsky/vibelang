# Choosing and combining schedules

`Schedule` works in Effeck exactly as in Effect. Start with the timing rule you need, add limits to it, and combine separate rules only when the job needs them. The consumer determines whether schedule input is a successful result (`repeat`) or an expected error (`retry`).

## Choosing a starting rule

```ts
import { Schedule } from "effect"

const immediate = Schedule.recurs(4)
const afterCompletion = Schedule.spaced("20 seconds")
const cadence = Schedule.fixed("20 seconds")
const backoff = Schedule.exponential("300 millis")
const calendar = Schedule.cron("15 4 * * *", "UTC")
```

These declarations build reusable values without running work. Choose a rule according to the requirement:

| Requirement | Rule | When it continues |
| --- | --- | --- |
| Four additional runs | `recurs(4)` | Immediately, up to four times |
| Pause between actions | `spaced` | After a full delay following completion |
| Keep a regular cadence | `fixed` | At the next interval boundary |
| Increase retry delays | `exponential` | With a growing wait, doubling by default |
| Follow a calendar | `cron` | At the next matching date in the given zone |

`Schedule.fibonacci` provides another growing backoff. None of the interval or backoff rules above stops by itself.

## Understanding schedule steps

```ts
import { Schedule } from "effect"

function runChecks() {
  Effect.repeat(() => {
    Effect.logInfo("checking")
    Effect.sleep("2 seconds")
  }, Schedule.upTo(Schedule.spaced("6 seconds"), { times: 2 }))
}
```

With `repeat`, the initial action runs before the schedule advances. This example has three action starts roughly at zero, eight, and sixteen seconds. Replacing `spaced` with `fixed` gives starts roughly at zero, eight, and fourteen seconds in this version: the fixed schedule anchors its cadence at its first step, after the initial two-second action. Later starts follow that cadence instead of adding a full delay after every action; overruns do not replay every missed interval.

A schedule step receives input and produces output plus timing metadata. In `retry`, each input is the latest error; in `repeat`, it is the latest success. Constructing one schedule does not share a running loop's step state with another loop.

## Limiting duration and adjusting delays

```ts
import { Duration, Schedule } from "effect"

const bounded = pipe(
  Schedule.exponential("300 millis"),
  Schedule.modifyDelay(({ duration }) =>
    Duration.min(duration, Duration.seconds(8))),
  Schedule.jittered,
  Schedule.upTo({ times: 6, duration: "40 seconds" })
)

const firstThree = Schedule.while(
  Schedule.spaced("1 second"),
  ({ attempt }) => attempt <= 3
)
```

`upTo` stops when either configured limit is reached. Its duration limit is checked at schedule steps; it does not interrupt a running attempt or wait, so use `Effect.timeout` around the whole operation for a deadline. `modifyDelay` replaces the selected wait, `jittered` spreads retries across time, and `while` can stop based on metadata.

## Requiring every timing rule

```ts
import { Schedule } from "effect"

const policy = Schedule.upTo(
  Schedule.max([
    Schedule.exponential("400 millis"),
    Schedule.spaced("2 seconds")
  ]),
  { times: 4 }
)
```

`max` continues only while every input schedule can continue and chooses the largest proposed delay. Here the first waits are two seconds, two seconds, two seconds, and 3.2 seconds. The separate limit permits four recurrences.

## Accepting any timing rule

```ts
import { Schedule } from "effect"

const startupAndSteady = Schedule.min([
  Schedule.upTo(Schedule.spaced("500 millis"), { times: 2 }),
  Schedule.fixed("12 seconds")
])
```

`min` continues while at least one rule remains active and chooses the shortest active delay. Both rules advance from the start, so the two quick probes do not restart the twelve second cadence. The combined schedule stops only when every input has stopped.

## Switching rules in phases

```ts
import { Schedule } from "effect"

const reconnect = Schedule.concat(
  Schedule.upTo(Schedule.spaced("150 millis"), { times: 2 }),
  Schedule.upTo(Schedule.spaced("3 seconds"), { times: 5 })
)
```

`concat` exhausts the first rule before starting the second. With `retry`, this permits an initial attempt, two fast retries, then five slow retries. `Schedule.concatResult` keeps the outputs of the two phases distinguishable when an observer needs to identify the active phase.
