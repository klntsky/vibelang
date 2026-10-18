# Option

`Option` works in VibeLang exactly as in Effect. It represents a value that is present (`Some<A>`) or absent (`None`), useful for lookups, optional fields, and validation that needs no error description.

## Representing optional values and properties

```vibe
// run: Console.log(named.nickname.getOrNull(), unnamed.nickname.isNone())
interface Contact {
  name: string
  nickname: Option.Option<string>
}

const named: Contact = { name: "Mina", nickname: Option.some("Mi") }
const unnamed: Contact = { name: "Sol", nickname: Option.none() }
```

```output
Mi true
```

The `nickname` property always exists; its value records presence or absence. An optional TypeScript property instead permits the key itself to be missing. Use the model that matches the distinction your program needs.

## Constructing and identifying variants

```vibe
// run: Console.log(positive(-2).isNone(), present.isSome(), absent.isNone()); Console.log(label(present)); Console.log(label(absent))
const present = Option.some(24)
const absent = Option.none<number>()
const positive = Option.liftPredicate((n: number) => n > 0)
positive(-2) // None
present.isSome() // true
absent.isNone() // true

function label(value: Option.Option<number>) {
  if (value.isSome()) return `Limit: ${value.value}`
  return "No limit"
}
```

```output
true true true
Limit: 24
No limit
```

`some` wraps a value, while `none` carries no payload. `liftPredicate` creates a function that returns `Some` only for values passing its predicate. Guards narrow the union so `.value` is available only for `Some`.

```vibe
// run: Console.log(describe(Option.some("Mina"))); Console.log(describe(Option.none()))
function describe(value: Option.Option<string>) {
  return value.match({
    onNone: () => "Anonymous",
    onSome: (name) => `Signed in as ${name}`
  })
}
```

```output
Signed in as Mina
Anonymous
```

`match` handles both cases and removes the wrapper. Its callbacks here only compute values; use an ordinary branch when handling presence requires services or other Effects.

## Mapping, sequencing, and filtering present values

```vibe
// run: Console.log(streetLabel(Option.some({ street: Option.some(" Main St ") })).getOrNull()); Console.log(streetLabel(Option.some({ street: Option.some(" ") })).isNone(), streetLabel(Option.none()).isNone())
interface Address {
  street: Option.Option<string>
}

function streetLabel(address: Option.Option<Address>) {
  const street = address.flatMap((value) => value.street)
  const trimmed = street.map((value) => value.trim())
  return trimmed.filter((value) => value.length > 0)
}
```

```output
Main St
true true
```

`map` transforms a present payload and skips its callback for `None`. `flatMap` sequences a callback returning another option, avoiding nested wrappers. `filter` keeps a present value only when the predicate passes.

## Extracting values with defaults or exceptions

```vibe
// run: Console.log(JSON.stringify([extract(Option.some("Mina")), extract(Option.none())])); Console.log(certain); const failed = Effect.exit(() => Option.none().getOrThrow()); if (failed.isFailure()) Console.log(failed.cause.hasDies(), failed.cause.hasFails())
function extract(value: Option.Option<string>) {
  return {
    defaulted: value.getOrElse(() => "Guest"),
    nullable: value.getOrNull(),
    optional: value.getOrUndefined()
  }
}

const certain = Option.some("known").getOrThrow()
```

```output
[{"defaulted":"Mina","nullable":"Mina","optional":"Mina"},{"defaulted":"Guest","nullable":null}]
known
true false
```

`getOrElse` computes its fallback only for `None`. `getOrNull` and `getOrUndefined` adapt to APIs expecting nullish values. `getOrThrow` throws a defect on `None`, which a VibeLang `catch` does not handle; prefer a guard, default, or `Effect.fromOption` when absence is expected rather than a programming error. A `catch` around only `getOrThrow` cannot run and receives warning EK2004 by default.

## Choosing fallback options

```vibe
// run: Console.log(choose(Option.some("preferred"), Option.some("backup")).getOrNull(), choose(Option.none(), Option.some("backup")).getOrNull(), first.getOrNull())
function choose(
  preferred: Option.Option<string>,
  backup: Option.Option<string>
) {
  return preferred.orElse(() => backup)
}

const first = Option.firstSomeOf([
  Option.none(), Option.some("regional"), Option.some("global")
]) // Some containing "regional"
```

```output
preferred backup regional
```

`orElse` evaluates its fallback callback only when the first option is absent. `firstSomeOf` selects the first present option from an iterable. Neither helper retries Effects: these are operations on optional data.

## Adapting nullable values

```vibe
// run: Console.log(Option.fromNullishOr(null).isNone(), Option.fromNullishOr(undefined).isNone(), Option.fromNullishOr(0).getOrNull(), JSON.stringify(Option.fromNullishOr("").getOrNull()))
Option.fromNullishOr(null) // None
Option.fromNullishOr(undefined) // None
Option.fromNullishOr(0) // Some containing 0
Option.fromNullishOr("") // Some containing ""
```

```output
true true 0 ""
```

`fromNullishOr` treats only `null` and `undefined` as absence. It preserves other falsy values, including zero, false, and the empty string. Convert back with `getOrNull` or `getOrUndefined` at an interop boundary.

## Converting options into effects

```vibe
// run: Console.log(JSON.stringify(requiredPair())); try { requiredName(Option.none()) } catch (error) { Console.log(error._tag) }
function requiredName(name: Option.Option<string>) {
  return Effect.fromOption(name)
}
// string!Cause.NoSuchElementError!

function requiredPair() {
  return all([
    Effect.fromOption(Option.some("east")),
    Effect.fromOption(Option.some(3))
  ])
}
```

```output
["east",3]
NoSuchElementError
```

`Effect.fromOption` produces the present value or fails with `NoSuchElementError` for `None`. Import that error from `Cause` when you need its type or guard. Option itself is not an Effect subtype, so conversion is explicit; calling a function that returns an option gives the option object.

## Combining options

```vibe
// run: const result = combine(Option.some("Mina"), Option.some(3)); Console.log(JSON.stringify([result.record.getOrNull(), result.tuple.getOrNull(), result.fields.getOrNull()])); const absent = combine(Option.none(), Option.some(3)); Console.log(absent.record.isNone(), absent.tuple.isNone(), absent.fields.isNone())
function combine(
  name: Option.Option<string>,
  count: Option.Option<number>
) {
  const record = name.zipWith(count, (name, count) => ({ name, count }))
  const tuple = Option.all([name, count])
  const fields = Option.all({ name, count })
  return { record, tuple, fields }
}
```

```output
[{"name":"Mina","count":3},["Mina",3],{"name":"Mina","count":3}]
true true true
```

`zipWith` combines two present values through a callback. `all` combines tuple, record, or iterable contents into a single option. If any required value is absent, the combined result is `None`.

## Combining several options

```ts
// TypeScript (.ts)
// combine-options.ts
// run: console.log(JSON.stringify(Option.getOrNull(combineOptions(Option.some(" Daily "), Option.some(3))))); console.log(Option.isNone(combineOptions(Option.none(), Option.some(3))))
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

```output
{"title":"Daily","total":3}
true
```

```vibe
// run: Console.log(JSON.stringify(combinedData().getOrNull()))
import { combineOptions } from "./combine-options.ts"

function combinedData() {
  return try(() => combineOptions(
    Option.some(" Daily "), Option.some(3)
  ))
}
```

```output
{"title":"Daily","total":3}
```

`Option.gen` sequences JavaScript generator yields and stops at the first `None`. In `.vibe`, `function*` creates a VibeLang producer instead, so generator-based APIs stay in `.ts` files; using `Option.gen` in `.vibe` is EK1002. Use `flatMap`, `zipWith`, and `all` in VibeLang, or keep generator syntax in a TypeScript helper. This unannotated helper uses an explicit boundary; a helper declared with `/** @vibelang trusted */` can be called directly.

## Deriving equality and ordering

```vibe
// run: Console.log(same(Option.some(4), Option.some(4)), same(Option.none(), Option.some(4))); Console.log(JSON.stringify([sorted.map((item) => item.getOrNull()), descending.map((item) => item.getOrNull())]))
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

```output
true false
[[null,2,8],[8,2,null]]
```

`makeEquivalence` compares present values using a chosen relation; two absent options are equivalent. `makeOrder` places `None` before `Some` and delegates present-value comparison to the supplied order. Flipping the complete option order moves absence to the end as well as reversing present values.
