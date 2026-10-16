# DateTime

`DateTime` works in EffectScript exactly as in Effect. It represents immutable instants, optionally carrying a time zone, and provides parsing, calendar operations, and formatting without mutating a JavaScript `Date`.

## Modeling UTC and zoned instants

```ts
import { DateTime } from "effect"

const instant = DateTime.makeUnsafe("2026-04-12T10:00:00Z")
const paris = DateTime.setZone(
  instant, DateTime.zoneMakeNamedUnsafe("Europe/Paris")
)
instant.epochMilliseconds === paris.epochMilliseconds // true
DateTime.formatIsoZoned(paris) // "2026-04-12T12:00:00.000+02:00[Europe/Paris]"
```

A `Utc` holds epoch milliseconds. A `Zoned` holds the same kind of instant plus a `TimeZone`: a fixed millisecond offset or a named region with daylight saving rules. Attaching a zone normally changes the calendar view, not the instant.

## Constructing timestamps from supported inputs

```ts
import { DateTime } from "effect"

const text = DateTime.make("2026-04-12T10:00:00Z")
const epoch = DateTime.make(0)
const parts = DateTime.make({ year: 2026, month: 4, day: 12 })
const date = DateTime.fromDateUnsafe(new Date("2026-04-12T10:00:00Z"))
const invalid = DateTime.make("not a timestamp") // None

const localAppointment = DateTime.makeZoned(
  { year: 2026, month: 4, day: 12, hour: 10 },
  { timeZone: "Europe/Paris", adjustForTimeZone: true }
)
const serialized = DateTime.makeZonedFromString(
  "2026-04-12T10:00:00.000+02:00[Europe/Paris]"
)
```

`DateTime.Input` accepts a `DateTime`, a JavaScript `Date`, epoch milliseconds, partial calendar parts, or a parseable string. `make` and `makeZoned` return an `Option`; `makeUnsafe`, `fromDateUnsafe`, and `makeZonedUnsafe` throw for invalid input. Use explicit offsets or UTC strings to avoid host-dependent parsing.

`adjustForTimeZone: true` interprets the supplied parts as wall-clock time in the selected zone. Without it, zoned construction attaches a zone to the input instant. Use the `disambiguation` option to choose how daylight saving gaps and repeated times are resolved.

## Reading current time

```ts
import { DateTime } from "effect"

function timestamp() {
  const now = DateTime.now()
  return DateTime.formatIso(now)
}
// Effect<string, never, never>
```

`now` is an Effect value: call it to read UTC time through Effect's Clock. Tests can control that clock. Effect also exports `nowUnsafe`, but it bypasses Clock; use `now()` for application time rather than `Date.now()` or `new Date()`.

## Identifying timestamp variants

```ts
import { DateTime } from "effect"

function describe(value: unknown) {
  if (!DateTime.isDateTime(value)) return "not a timestamp"
  if (DateTime.isUtc(value)) return "UTC"
  if (DateTime.isZoned(value)) {
    if (DateTime.isTimeZoneNamed(value.zone)) return value.zone.id
    if (DateTime.isTimeZoneOffset(value.zone)) return String(value.zone.offset)
  }
  return "unknown"
}
```

`isDateTime` narrows unknown input, and `isUtc` and `isZoned` distinguish its variants. `isTimeZone` recognizes a zone; `isTimeZoneNamed` and `isTimeZoneOffset` expose its region identifier or fixed offset.

## Parsing and changing time zones

```ts
import { DateTime } from "effect"

const named = DateTime.zoneMakeNamed("Asia/Tokyo")
const parsed = DateTime.zoneFromString("+09:00")
const fixed = DateTime.zoneMakeOffset(9 * 60 * 60 * 1000)
DateTime.zoneToString(fixed) // "+09:00"

function inTokyo() {
  const zone = DateTime.zoneMakeNamedEffect("Asia/Tokyo")
  const now = DateTime.now()
  return DateTime.setZone(now, zone)
}

const base = DateTime.makeUnsafe("2026-04-12T10:00:00Z")
const optional = DateTime.setZoneNamed(base, "Asia/Tokyo")
const known = DateTime.setZoneNamedUnsafe(base, "Asia/Tokyo")
const shiftedView = DateTime.setZoneOffset(base, 9 * 60 * 60 * 1000)
```

`zoneMakeNamed` and `zoneFromString` return `None` for invalid identifiers. `zoneMakeNamedEffect` raises `IllegalArgumentError` as an expected failure, while `zoneMakeNamedUnsafe` throws. `zoneMakeLocal` reads the host's local zone; prefer a configured zone when output must be reproducible.

## Comparing and converting instants

```ts
import { DateTime, Duration } from "effect"

const start = DateTime.makeUnsafe("2026-04-12T10:00:00Z")
const end = DateTime.add(start, { hours: 3 })
DateTime.isLessThan(start, end) // true
DateTime.min(start, end) // start
DateTime.max(start, end) // end
Duration.toHours(DateTime.distance(start, end)) // 3
DateTime.toEpochMillis(start) // numeric Unix timestamp
const native = DateTime.toDateUtc(start)
const calendarDate = DateTime.removeTime(start)
```

Ordering compares instants, independent of the attached zone. `distance(start, end)` is signed: reversing the arguments produces a negative duration. `toDateUtc` preserves the instant in a JavaScript `Date`; `toDate` represents the zone-adjusted calendar view instead, so do not use it as an interchangeable instant conversion.

`isGreaterThan`, `isGreaterThanOrEqualTo`, `isLessThanOrEqualTo`, and inclusive `between` cover other comparisons. `isFuture` and `isPast` return Effects that read Clock. For zoned values, `zonedOffset` and `zonedOffsetIso` return the effective offset at that instant.

## Accessing calendar components

```ts
import { DateTime } from "effect"

const evening = DateTime.makeZonedUnsafe("2026-04-12T22:30:00Z", {
  timeZone: "Asia/Tokyo"
})
DateTime.getPart(evening, "day") // 13
DateTime.getPartUtc(evening, "day") // 12
const localParts = DateTime.toParts(evening)
const utcParts = DateTime.toPartsUtc(evening)
const rescheduled = DateTime.setParts(evening, { hour: 9, minute: 0 })
const utcEdited = DateTime.setPartsUtc(evening, { hour: 9 })
```

`Parts` contains year, month, day, hour, minute, second, and millisecond; months start at one. `PartsWithWeekday` adds `weekDay`. Use the UTC variants when calendar interpretation must ignore an attached zone.

## Calendar arithmetic and formatting

```ts
import { DateTime, Duration } from "effect"

const appointment = DateTime.makeZonedUnsafe("2026-04-12T10:00:00Z", {
  timeZone: "Europe/Paris"
})
const tomorrow = DateTime.add(appointment, { days: 1 })
const previous = DateTime.subtract(appointment, { months: 1 })
const elapsedHour = DateTime.addDuration(appointment, Duration.hours(1))
const earlier = DateTime.subtractDuration(appointment, Duration.minutes(15))
const dayStart = DateTime.startOf(appointment, "day")
const dayEnd = DateTime.endOf(appointment, "day")
const rounded = DateTime.nearest(appointment, "hour")
DateTime.formatIso(tomorrow) // "2026-04-13T10:00:00.000Z"
DateTime.formatIsoDate(appointment) // "2026-04-12"
DateTime.formatIsoDateUtc(appointment) // "2026-04-12"
DateTime.formatIsoOffset(appointment) // "2026-04-12T12:00:00.000+02:00"
DateTime.format(appointment, { locale: "en-GB", dateStyle: "long" })
```

Calendar `add` and `subtract` operate on parts in the value's zone; duration arithmetic adds fixed elapsed time. These differ across daylight saving transitions. Use `formatIsoZoned` to retain a named zone, `formatUtc` to force UTC presentation, `formatLocal` for host-local presentation, or `formatIntl` with an existing formatter.

## Providing a default time zone

```ts
import { DateTime } from "effect"

function localDate() {
  return DateTime.formatIsoDate(DateTime.nowInCurrentZone())
}
// Effect<string, never, DateTime.CurrentTimeZone>

function main() {
  return Effect.provide(
    () => localDate(),
    DateTime.layerCurrentZoneNamed("Asia/Tokyo")
  )
}
```

`CurrentTimeZone` supplies a zone for `nowInCurrentZone` and `setZoneCurrent`. Provide it with `layerCurrentZone`, `layerCurrentZoneNamed`, `layerCurrentZoneOffset`, or `layerCurrentZoneLocal`; the named layer can fail if its identifier is invalid. The `withCurrentZone`, `withCurrentZoneNamed`, `withCurrentZoneOffset`, and `withCurrentZoneLocal` combinators provide the same setting around a program, passed as `() => work()`.
