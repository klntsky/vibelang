# Duration

`Duration` works in Effeck exactly as in Effect. Use it to express elapsed time, delays, and timeout budgets with explicit units; Effect 4.0.5 also supports negative durations and infinities.

## Constructing finite and infinite durations

```ts
import { Duration } from "effect"

const small = Duration.nanos(750n)
const micro = Duration.micros(20n)
const polling = Duration.millis(250)
const budget = Duration.seconds(12)
const session = Duration.minutes(15)
const shift = Duration.hours(8)
const retention = Duration.days(3)
const fortnight = Duration.weeks(2)
const unbounded = Duration.infinity
const negative = Duration.seconds(-2)
```

Constructors identify the unit at the call site. `infinity` represents an unbounded positive span; `negativeInfinity` represents its negative counterpart. A day here is a fixed 24 hours, not a calendar-day operation across daylight saving changes.

```ts
import { Duration } from "effect"

const milliseconds = Duration.fromInputUnsafe(240)
const nanoseconds = Duration.fromInputUnsafe(240n)
const text = Duration.fromInputUnsafe("3 seconds")
const checked = Duration.fromInput("3 seconds")
```

`fromInputUnsafe` accepts durations, numeric milliseconds, bigint nanoseconds, and supported duration strings. It throws for invalid input; `fromInput` returns an `Option` instead. Prefer a unit constructor when the unit is known.

## Extracting values in chosen units

```ts
import { Duration } from "effect"

const delay = Duration.minutes(2)
Duration.toMillis(delay) // 120000
Duration.toSeconds(delay) // 120
Duration.toMinutes(delay) // 2
Duration.toHours(delay) // 1 / 30
Duration.toDays(Duration.weeks(1)) // 7
Duration.toWeeks(Duration.days(14)) // 2
Duration.toNanos(Duration.millis(3)) // Some containing 3000000n
Duration.toNanos(Duration.infinity) // None
```

Numeric conversions return numbers in the requested unit. `toNanos` returns an `Option<bigint>` because infinity has no bigint representation. `toNanosUnsafe` returns the bigint directly for finite input and throws for infinite input.

## Comparing durations

```ts
import { Duration } from "effect"

const elapsed = Duration.seconds(9)
const budget = Duration.seconds(12)
Duration.isLessThan(elapsed, budget) // true
Duration.isLessThanOrEqualTo(elapsed, elapsed) // true
Duration.isGreaterThan(budget, elapsed) // true
Duration.isGreaterThanOrEqualTo(budget, budget) // true
Duration.equals(Duration.seconds(1), Duration.millis(1000)) // true
```

Comparison uses the elapsed span, independent of the constructor used. Keep durations in this form rather than converting them all to a less precise unit before comparing.

## Adding and scaling durations

```ts
import { Duration } from "effect"

const attempt = Duration.seconds(4)
const overhead = Duration.millis(500)
const total = Duration.sum(attempt, overhead)
const doubled = Duration.times(total, 2)
Duration.toMillis(doubled) // 9000

function waitBeforeRetry() {
  Effect.sleep(doubled)
}
```

`sum` adds spans and `times` scales by a numeric factor. Both return new values. `Effect.sleep` accepts the resulting duration and waits when called inside a program.

## Converting and formatting

```ts
import { Duration } from "effect"

Duration.format(Duration.millis(6250)) // "6s 250ms"
String(Duration.infinity) // "Infinity"
Duration.toNanosUnsafe(Duration.millis(2)) // 2000000n
```

Use `format` for a compact display string and explicit unit conversions for calculations. The formatted text is a presentation choice, so store a defined numeric unit when a wire protocol requires one.
