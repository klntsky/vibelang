# Duration

`Duration` works in VibeLang exactly as in Effect. Use it to express elapsed time, delays, and timeout budgets with explicit units; Effect 4.0.5 also supports negative durations and infinities.

## Constructing finite and infinite durations

```vibe
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

```vibe
const milliseconds = Duration.fromInputUnsafe(240)
const nanoseconds = Duration.fromInputUnsafe(240n)
const text = Duration.fromInputUnsafe("3 seconds")
const checked = Duration.fromInput("3 seconds")
```

`fromInputUnsafe` accepts durations, numeric milliseconds, bigint nanoseconds, and supported duration strings. It throws for invalid input; `fromInput` returns an `Option` instead. Prefer a unit constructor when the unit is known.

## Extracting values in chosen units

```vibe
const delay = Duration.minutes(2)
delay.toMillis() // 120000
delay.toSeconds() // 120
delay.toMinutes() // 2
delay.toHours() // 1 / 30
Duration.weeks(1).toDays() // 7
Duration.days(14).toWeeks() // 2
Duration.millis(3).toNanos() // Some containing 3000000n
Duration.infinity.toNanos() // None
```

Numeric conversions return numbers in the requested unit. `toNanos` returns an `Option<bigint>` because infinity has no bigint representation. `toNanosUnsafe` returns the bigint directly for finite input and throws for infinite input.

## Comparing durations

```vibe
const elapsed = Duration.seconds(9)
const budget = Duration.seconds(12)
elapsed.isLessThan(budget) // true
elapsed.isLessThanOrEqualTo(elapsed) // true
budget.isGreaterThan(elapsed) // true
budget.isGreaterThanOrEqualTo(budget) // true
Duration.seconds(1).equals(Duration.millis(1000)) // true
```

Comparison uses the elapsed span, independent of the constructor used. Keep durations in this form rather than converting them all to a less precise unit before comparing.

## Adding and scaling durations

```vibe
const attempt = Duration.seconds(4)
const overhead = Duration.millis(500)
const total = attempt.sum(overhead)
const doubled = total.times(2)
doubled.toMillis() // 9000

function waitBeforeRetry() {
  sleep(doubled)
}
```

`sum` adds spans and `times` scales by a numeric factor. Both return new values. `Effect.sleep` accepts the resulting duration and waits when called inside a program.

## Converting and formatting

```vibe
Duration.millis(6250).format() // "6s 250ms"
String(Duration.infinity) // "Infinity"
Duration.toNanosUnsafe(Duration.millis(2)) // 2000000n
```

Use `format` for a compact display string and explicit unit conversions for calculations. The formatted text is a presentation choice, so store a defined numeric unit when a wire protocol requires one.
