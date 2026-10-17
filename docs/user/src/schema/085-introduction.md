# Introduction to Effect Schema

`Schema` works in Effeck exactly as in Effect: it describes values, checks data, and converts between representations. Use a schema at a boundary where incoming data has an unknown shape, or where a domain value needs a storage or transport format. `Schema` is already in scope in `.effeck` files.

## Configuring compiler prerequisites

```ts
// settings.effeck
const Settings = Schema.Struct({
  label: Schema.String,
  hint: Schema.optionalKey(Schema.String)
})

function readSettings(input: unknown) {
  return Schema.decodeUnknownEffect(Settings)(input)
}
// typeof Settings.Type!Schema.SchemaError!
```

Enable `strict` in the project's compiler options. Enable `exactOptionalPropertyTypes` as well to distinguish an absent `hint` from a present `hint: undefined`. `optionalKey` accepts the absent key but rejects explicit `undefined`; turning the compiler option off does not change that runtime rule. Effeck v0 uses the TypeScript 7.1 checker and supports `effect@4.0.5` exactly. Install `effeck-runtime` and `effect@4.0.5` as dependencies and `effeck` as a dev dependency. Run `effeck --noEmit` to check `.effeck` and `.ts` files together; `effeck` also emits JavaScript and declarations.

Effect's own packages are trusted. Synchronous Schema builders, `.pipe` methods, guards, and generated helpers are plain calls; their exceptions are defects. `catch` does not handle those defects. Use an Effect-returning parser for typed validation failures, or an explicit `try` hatch to give a synchronous exception an expected error type. External plain TypeScript functions need a boundary or a `/** @effeck trusted */` contract inside a program.

## Understanding schemas and codecs

```ts
const Quantity = Schema.FiniteFromString
const Line = Schema.Struct({ sku: Schema.NonEmptyString, quantity: Quantity })

// Views of the same value, retaining different information:
const decodedView: Schema.Schema<typeof Line.Type> = Line
const completeView: Schema.Codec<typeof Line.Type, typeof Line.Encoded> = Line
```

A schema is an immutable description. Adding fields, checks, or annotations produces a new description. Interpreters use that description to derive parsers, guards, generators, equality, formatters, and JSON Schema documents.

`Schema.Schema<T>` retains only the decoded type. `Schema.Decoder<T, RD>` also retains decoding services; `Schema.Encoder<E, RE>` retains the encoded type and encoding services. `Schema.Codec<T, E, RD, RE>` retains all four, while `Schema.Top` accepts any schema without preserving a particular value type.

## Distinguishing decoded and encoded views

```ts
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

Decoding goes from `Encoded` to `Type`; encoding goes from `Type` to `Encoded`. In this example, the application uses numbers and the external representation uses strings. The codec tracks services independently in each direction, with `never` meaning that direction needs no services.

## Decoding or encoding unknown inputs

```ts
function inspectBoundary(incoming: unknown, outgoing: unknown) {
  const quantity = Schema.decodeUnknownEffect(Schema.FiniteFromString)(incoming)
  const text = Schema.encodeUnknownEffect(Schema.FiniteFromString)(outgoing)
  return { quantity, text }
}
// { quantity: number; text: string }!Schema.SchemaError!
```

The `Unknown` interpreters validate the input before converting it. Decoding expects a string here and produces a finite number; encoding expects a finite number and produces a string. The typed `decodeEffect` and `encodeEffect` variants constrain their arguments statically and still validate them at runtime.

Calling an Effect-returning parser runs it. The result is the decoded or encoded value, and a schema mismatch enters the enclosing function's error channel.

## Reasoning about conversion round trips

```ts
function roundTrip(value: number) {
  const wire = Schema.encodeEffect(Schema.FiniteFromString)(value)
  return Schema.decodeEffect(Schema.FiniteFromString)(wire)
}

function normalizeText() {
  const value = Schema.decodeEffect(Schema.Trim)("  dispatch  ")
  return Schema.encodeEffect(Schema.Trim)(value) // "dispatch"
}
```

For a valid domain value, aim for `decode(encode(value))` to be equivalent to `value`. This is a property of the chosen conversions, not an automatic guarantee. The other direction can normalize data: whitespace, numeric spelling, and discarded fields need not survive an encode after a decode.

## Coming from Effect

Build schemas with the same Effect 4 constructors. Replace the generator around a parser with an ordinary function; its parser calls run directly, and `try`/`catch` handles `SchemaError` failures.

[Upstream page](https://effect.website/docs/v4/schema/introduction)
