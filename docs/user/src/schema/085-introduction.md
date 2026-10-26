# Introduction to Effect Schema

`Schema` works exactly as in Effect: it describes values, checks data, and converts between representations. Use one at a boundary where incoming data has an unknown shape, or where a domain value needs a storage or transport format. `.vibe` files need no import for `Schema`.

## Configuring compiler prerequisites

```vibe
// settings.vibe
// run: Console.log(JSON.stringify(readSettings({ label: "depot" }))); Console.log(Schema.is(Settings)({ label: "depot", hint: undefined }))
import { Console } from "effect"
const Settings = Schema.Struct({
  label: Schema.String,
  hint: Schema.optionalKey(Schema.String)
})

function readSettings(input: unknown) {
  return Schema.decodeUnknownEffect(Settings)(input)
}
// Effect<typeof Settings.Type, Schema.SchemaError>
```

```output
{"label":"depot"}
false
```

Enable `strict` in the compiler options, and `exactOptionalPropertyTypes` to distinguish an absent `hint` from a present `hint: undefined`. At runtime `optionalKey` accepts the absent key and rejects explicit `undefined`, whatever the compiler option. VibeLang v0 uses the TypeScript 7.1 checker and supports exactly `effect@4.0.5`. Install `vibelang-runtime` and `effect@4.0.5` as dependencies and `vibelang` as a dev dependency. `vibelang --noEmit` checks `.vibe` and `.ts` files together; `vibelang` also emits JavaScript and declarations.

Effect's own packages are trusted: synchronous Schema builders, `.pipe` methods, guards, and generated helpers are plain calls, and an exception escaping into Effect code is a defect that Effect catch does not handle. A plain function's catch doesn't see it either (D282). For typed validation failures, use an Effect-returning parser; to give a synchronous exception an expected error type, use an explicit `try` hatch. Inside a program, an external plain TypeScript function needs an explicit boundary or a `trusted(...)` adapter from `vibelang-runtime`.

## Understanding schemas and codecs

```vibe
// run: Console.log(decodedView === Line, completeView === Line)
const Quantity = Schema.FiniteFromString
const Line = Schema.Struct({ sku: Schema.NonEmptyString, quantity: Quantity })

// Views of the same value, retaining different information:
const decodedView: Schema.Schema<typeof Line.Type> = Line
const completeView: Schema.Codec<typeof Line.Type, typeof Line.Encoded> = Line
```

```output
true true
```

A schema is an immutable description; adding fields, checks, or annotations produces a new one. Interpreters derive parsers, guards, generators, equality, formatters, and JSON Schema documents from it.

`Schema.Schema<T>` retains only the decoded type; `Schema.Decoder<T, RD>` adds decoding services; `Schema.Encoder<E, RE>` retains the encoded type and encoding services; `Schema.Codec<T, E, RD, RE>` retains all four. `Schema.Top` accepts any schema without preserving a particular value type.

## Distinguishing decoded and encoded views

```vibe
// run: Console.log(JSON.stringify(unpack())); Console.log(JSON.stringify(pack({ weight: 3.75 })))
const Parcel = Schema.Struct({ weight: Schema.FiniteFromString })

type Parcel = typeof Parcel.Type       // { readonly weight: number }
type ParcelWire = typeof Parcel.Encoded // { readonly weight: string }

function unpack() {
  return Schema.decodeEffect(Parcel)({ weight: "3.75" })
}

function pack(parcel: Parcel) {
  return Schema.encodeEffect(Parcel)(parcel)
}
```

```output
{"weight":3.75}
{"weight":"3.75"}
```

Decoding goes from `Encoded` to `Type`, encoding from `Type` to `Encoded`. The codec tracks services separately per direction; `never` means that direction needs none.

## Decoding or encoding unknown inputs

```vibe
// run: Console.log(JSON.stringify(inspectBoundary("3.75", 2))); try { inspectBoundary(3, "2") } catch (error) { Console.log(error._tag) }
function inspectBoundary(incoming: unknown, outgoing: unknown) {
  const quantity = Schema.decodeUnknownEffect(Schema.FiniteFromString)(incoming)
  const text = Schema.encodeUnknownEffect(Schema.FiniteFromString)(outgoing)
  return { quantity, text }
}
// Effect<{ quantity: number; text: string }, Schema.SchemaError>
```

```output
{"quantity":3.75,"text":"2"}
SchemaError
```

The `Unknown` interpreters validate the input before converting it: here decoding takes a string and produces a finite number, and encoding the reverse. The typed `decodeEffect` and `encodeEffect` constrain their arguments statically and still validate them at runtime.

Calling an Effect-returning parser runs it: it returns the decoded or encoded value, and a schema mismatch enters the enclosing function's error channel.

## Reasoning about conversion round trips

```vibe
// run: Console.log(roundTrip(3.75)); Console.log(normalizeText())
function roundTrip(value: number) {
  const wire = Schema.encodeEffect(Schema.FiniteFromString)(value)
  return Schema.decodeEffect(Schema.FiniteFromString)(wire)
}

function normalizeText() {
  const value = Schema.decodeEffect(Schema.Trim)("  dispatch  ")
  return Schema.encodeEffect(Schema.Trim)(value) // "dispatch"
}
```

```output
3.75
dispatch
```

For a valid domain value, aim for `decode(encode(value))` to be equivalent to `value`; the chosen conversions decide this, and nothing guarantees it. The other direction can normalize: whitespace, numeric spelling, and discarded fields need not survive an encode after a decode.

## Coming from Effect

Schemas use the same Effect 4 constructors. An ordinary function replaces the generator around a parser: parser calls run directly, and `try`/`catch` handles `SchemaError` failures.

[Upstream page](https://effect.website/docs/v4/schema/introduction)
