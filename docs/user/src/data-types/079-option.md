# Option

`Option` works in Effeck exactly as in Effect. It represents a value that is present (`Some<A>`) or absent (`None`), useful for lookups, optional fields, and validation that needs no error description.

## Representing optional values and properties

```ts
interface Contact {
  readonly name: string
  readonly nickname: Option.Option<string>
}

const named: Contact = { name: "Mina", nickname: Option.some("Mi") }
const unnamed: Contact = { name: "Sol", nickname: Option.none() }
```

The `nickname` property always exists; its value records presence or absence. An optional TypeScript property instead permits the key itself to be missing. Use the model that matches the distinction your program needs.

## Constructing and identifying variants

```ts
const present = Option.some(24)
const absent = Option.none<number>()
const positive = Option.liftPredicate((n: number) => n > 0)
positive(-2) // None
Option.isSome(present) // true
Option.isNone(absent) // true

function label(value: Option.Option<number>) {
  if (Option.isSome(value)) return `Limit: ${value.value}`
  return "No limit"
}
```

`some` wraps a value, while `none` carries no payload. `liftPredicate` creates a function that returns `Some` only for values passing its predicate. Guards narrow the union so `.value` is available only for `Some`.

```ts
function describe(value: Option.Option<string>) {
  return Option.match(value, {
    onNone: () => "Anonymous",
    onSome: (name) => `Signed in as ${name}`
  })
}
```

`match` handles both cases and removes the wrapper. Its callbacks here only compute values; use an ordinary branch when handling presence requires services or other Effects.

## Mapping, sequencing, and filtering present values

```ts
interface Address {
  readonly street: Option.Option<string>
}

function streetLabel(address: Option.Option<Address>) {
  const street = Option.flatMap(address, (value) => value.street)
  const trimmed = Option.map(street, (value) => value.trim())
  return Option.filter(trimmed, (value) => value.length > 0)
}
```

`map` transforms a present payload and skips its callback for `None`. `flatMap` sequences a callback returning another option, avoiding nested wrappers. `filter` keeps a present value only when the predicate passes.

## Extracting values with defaults or exceptions

```ts
function extract(value: Option.Option<string>) {
  return {
    defaulted: Option.getOrElse(value, () => "Guest"),
    nullable: Option.getOrNull(value),
    optional: Option.getOrUndefined(value)
  }
}

const certain = Option.getOrThrow(Option.some("known"))
```

`getOrElse` computes its fallback only for `None`. `getOrNull` and `getOrUndefined` adapt to APIs expecting nullish values. `getOrThrow` throws a defect on `None`, which an Effeck `catch` does not handle; prefer a guard, default, or `Effect.fromOption` when absence is expected rather than a programming error. A `catch` around only `getOrThrow` cannot run and receives warning EK2004 by default.

## Choosing fallback options

```ts
function choose(
  preferred: Option.Option<string>,
  backup: Option.Option<string>
) {
  return Option.orElse(preferred, () => backup)
}

const first = Option.firstSomeOf([
  Option.none(), Option.some("regional"), Option.some("global")
]) // Some containing "regional"
```

`orElse` evaluates its fallback callback only when the first option is absent. `firstSomeOf` selects the first present option from an iterable. Neither helper retries Effects: these are operations on optional data.

## Adapting nullable values

```ts
Option.fromNullishOr(null) // None
Option.fromNullishOr(undefined) // None
Option.fromNullishOr(0) // Some containing 0
Option.fromNullishOr("") // Some containing ""
```

`fromNullishOr` treats only `null` and `undefined` as absence. It preserves other falsy values, including zero, false, and the empty string. Convert back with `getOrNull` or `getOrUndefined` at an interop boundary.

## Converting options into effects

```ts
function requiredName(name: Option.Option<string>) {
  return Effect.fromOption(name)
}
// string!Cause.NoSuchElementError!

function requiredPair() {
  return Effect.all([
    Effect.fromOption(Option.some("east")),
    Effect.fromOption(Option.some(3))
  ])
}
```

`Effect.fromOption` produces the present value or fails with `NoSuchElementError` for `None`. Import that error from `Cause` when you need its type or guard. Option itself is not an Effect subtype, so conversion is explicit; calling a function that returns an option gives the option object.

## Combining options

```ts
function combine(
  name: Option.Option<string>,
  count: Option.Option<number>
) {
  const record = Option.zipWith(name, count, (name, count) => ({ name, count }))
  const tuple = Option.all([name, count])
  const fields = Option.all({ name, count })
  return { record, tuple, fields }
}
```

`zipWith` combines two present values through a callback. `all` combines tuple, record, or iterable contents into a single option. If any required value is absent, the combined result is `None`.

## Combining options with generators

```ts
// TypeScript (.ts)
// combine-options.ts
import { Option } from "effect"

export function combineOptions(
  name: Option.Option<string>,
  count: Option.Option<number>
) {
  return Option.gen(function* () {
    const title = (yield* name).trim()
    const total = yield* count
    return { title, total }
  })
}
```

```ts
import { combineOptions } from "./combine-options.ts"

function combinedData() {
  return try(() => combineOptions(
    Option.some(" Daily "), Option.some(3)
  ))
}
```

`Option.gen` sequences JavaScript generator yields and stops at the first `None`. In `.effeck`, `function*` creates an Effeck producer instead, so generator-based APIs stay in `.ts` files; using `Option.gen` in `.effeck` is EK1002. Use `flatMap`, `zipWith`, and `all` in Effeck, or keep generator syntax in a TypeScript helper. This unannotated helper uses an explicit boundary; a helper declared with `/** @effeck trusted */` can be called directly.

## Deriving equality and ordering

```ts
import { Array as Arr } from "effect"
const same = Option.makeEquivalence(Equivalence.Number)
same(Option.some(4), Option.some(4)) // true
same(Option.none(), Option.some(4)) // false

const ascending = Option.makeOrder(Order.Number)
const items = [Option.some(8), Option.none(), Option.some(2)]
const sorted = Arr.sort(items, ascending)
// [None, Some(2), Some(8)]
const descending = Arr.sort(items, Order.flip(ascending))
// [Some(8), Some(2), None]
```

`makeEquivalence` compares present values using a chosen relation; two absent options are equivalent. `makeOrder` places `None` before `Some` and delegates present-value comparison to the supplied order. Flipping the complete option order moves absence to the end as well as reversing present values.
