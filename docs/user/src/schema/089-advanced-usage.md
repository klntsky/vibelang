# Advanced Usage

The advanced Schema APIs work as in Effect. Use declarations for opaque runtime values, brands for static distinctions, field combinators for presence rules, and suspension for recursive descriptions. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Declaring custom scalar and parameterized schemas

```ts
class Shelf {
  constructor(readonly code: string) {}
}

const ShelfSchema = Schema.declare(
  (input: unknown) => input instanceof Shelf,
  { identifier: "Shelf", description: "A warehouse shelf handle" }
)

function readShelf(input: unknown) {
  return Schema.decodeUnknownEffect(ShelfSchema)(input)
}
```

A declaration's guard establishes identity; its type predicate is inferred from `instanceof`. It does not validate every member automatically. Add checks when fields need validation. Unlike a struct, an opaque declaration gives interpreters no structural description to traverse.

```ts
class Box<A> {
  constructor(readonly value: A) {}
}

function BoxSchema<S extends Schema.Constraint>(value: S) {
  return Schema.declareConstructor<Box<S["Type"]>, Box<S["Encoded"]>>()(
    [value],
    ([element]) => (input, ast, options) => {
      if (!(input instanceof Box)) throw new SchemaIssue.InvalidType(ast)
      const decoded = SchemaParser.decodeUnknownEffect(element)(input.value, options)
      return new Box(decoded)
    },
    { expected: "Box" }
  )
}

const BoxedCount = BoxSchema(Schema.FiniteFromString)
```

`declareConstructor` passes codecs for the type parameters to its parser factory. The parser returns the declared decoded value or fails with a raw `SchemaIssue.Issue`; the outer Schema interpreter adds a `SchemaError` wrapper. Encoding reuses the declaration machinery with the directional parameter codecs. Additional service requirements must come from the parameter schemas; the declaration parser cannot introduce an unrelated service.

## Adding interpreter support

```ts
const PortableShelf = Schema.declare(
  (input: unknown) => input instanceof Shelf,
  {
    identifier: "PortableShelf",
    toCodec: () => Schema.link<Shelf>()(
      Schema.NonEmptyString,
      SchemaTransformation.transform({
        decode: (code) => new Shelf(code),
        encode: (shelf) => shelf.code
      })
    ),
    toCodecArbitrary: () => Schema.link<Shelf>()(
      Schema.NonEmptyString,
      SchemaTransformation.transform({
        decode: (code) => new Shelf(code),
        encode: (shelf) => shelf.code
      })
    ),
    toFormatter: () => (shelf) => `Shelf(${shelf.code})`,
    toEquivalence: () => (left, right) => left.code === right.code
  }
)
```

`toCodec` supplies a shared structural representation for canonical codecs. Use `toCodecJson`, `toCodecStringTree`, or `toCodecIso` when a format needs its own representation. `toCodecArbitrary` supplies a generatable representation, and formatter and equivalence annotations supply behavior for those interpreters. For parameterized declarations, interpreter callbacks receive the corresponding parameter interpretations.

## Creating branded schemas

```ts
const ShelfId = Schema.NonEmptyString.pipe(Schema.brand("ShelfId"))
type ShelfId = typeof ShelfId.Type

type TrackingId = string & Brand.Brand<"TrackingId">
const TrackingId = Brand.nominal<TrackingId>()
const TrackingIdSchema = Schema.String.pipe(Schema.fromBrand("TrackingId", TrackingId))

```

A brand distinguishes values statically without changing their runtime representation. A brand alone adds no check; combine it with a validating schema. `fromBrand` reuses an existing branded constructor.

## Applying constructor defaults

```ts
const Parcel = Schema.Struct({
  shelf: ShelfId,
  priority: Schema.Int.pipe(Schema.withConstructorDefault(() => 1))
})

function createParcel() {
  const shelf = ShelfId.makeEffect("rack-18")
  return Parcel.makeEffect({ shelf })
}
```

`withConstructorDefault` makes a field optional for construction and evaluates its program when needed. It does not make that field optional during decoding. Constructor failures are raw schema issues in local Effect 4.0.5.

## Distinguishing optional keys, optional values, null, and absence

```ts
const Presence = Schema.Struct({
  exact: Schema.optionalKey(Schema.String),
  loose: Schema.optional(Schema.String),
  requiredUndefined: Schema.UndefinedOr(Schema.String),
  requiredNullable: Schema.NullOr(Schema.String),
  prohibited: Schema.optionalKey(Schema.Never)
})
const WithDecodeDefault = Schema.Struct({
  copies: Schema.FiniteFromString.pipe(Schema.withDecodingDefaultType(() => 1)),
  rank: Schema.FiniteFromString.pipe(Schema.withDecodingDefaultTypeKey(() => 0))
})
```

| Field form | Missing key | Explicit `undefined` | Explicit `null` |
| --- | --- | --- | --- |
| `optionalKey(String)` | accepted | rejected | rejected |
| `optional(String)` | accepted | accepted | rejected |
| `UndefinedOr(String)` | rejected | accepted | rejected |
| `NullOr(String)` | rejected | rejected | accepted |
| `optionalKey(Never)` | accepted | rejected | rejected |

`withDecodingDefaultType` applies a decoded default to a missing or undefined input; the `Key` variant applies it only to a missing key. `withDecodingDefault` and `withDecodingDefaultKey` instead accept defaults in the encoded representation. None of these decoding defaults implies a constructor default. Enable `exactOptionalPropertyTypes` to keep these distinctions visible in static types.

## Mapping optional fields to options

```ts
const Preferences = Schema.Struct({
  note: Schema.OptionFromOptionalKey(Schema.String),
  weight: Schema.OptionFromOptional(Schema.FiniteFromString),
  alias: Schema.OptionFromOptionalNullOr(Schema.String)
})

function readPreferences() {
  return Schema.decodeUnknownEffect(Preferences)({ weight: undefined, alias: null })
}
// All three fields contain Option.none().
```

The decoded keys are required and hold `Option` values. The exact-key variant rejects explicit `undefined`; `OptionFromOptional` treats it as absent, and `OptionFromOptionalNullOr` also treats null as absent. By default, encoding `None` omits the key. For a required value containing null or undefined, use the corresponding `OptionFromNullOr`, `OptionFromUndefinedOr`, or `OptionFromNullishOr` instead.

## Transforming field presence

```ts
const Form = Schema.Struct({
  note: Schema.optionalKey(Schema.String).pipe(
    Schema.decodeTo(Schema.optionalKey(Schema.String), {
      decode: SchemaGetter.transformOptional((value) =>
        value.isSome() && value.value === "" ? Option.none() : value
      ),
      encode: SchemaGetter.transformOptional((value) => value)
    })
  ),
  label: Schema.optionalKey(Schema.String).pipe(
    Schema.decodeTo(Schema.NullOr(Schema.String), {
      decode: SchemaGetter.transformOptional((value) =>
        value.isNone() ? Option.some(null) : value
      ),
      encode: SchemaGetter.transformOptional((value) =>
        value.isNone() || value.value === null ? Option.none() : Option.some(value.value)
      )
    })
  ),
  memo: Schema.String.pipe(
    Schema.decodeTo(Schema.optionalKey(Schema.String), {
      decode: SchemaGetter.transformOptional((value) =>
        value.isSome() && value.value === "" ? Option.none() : value
      ),
      encode: SchemaGetter.transformOptional((value) =>
        value.isNone() ? Option.some("") : value
      )
    })
  )
})
```

Optional getters receive an `Option`: `None` means that the key is absent, while `Some(undefined)` means it is present with an undefined value. Returning `None` removes a key. `note` demonstrates optional-to-optional conversion, `label` supplies a required nullable output, and `memo` turns a required empty string into an absent decoded field while restoring it during encoding. Filter a nullable value out of the input Option when null should count as absence.

## Extending and renaming fields

```ts
const Address = Schema.Struct({ city: Schema.String })
const Destination = Schema.Struct({ ...Address.fields, dock: Schema.Int })
const Routed = Destination.pipe(Schema.fieldsAssign({ route: Schema.String }))
const WireDestination = Routed.pipe(Schema.encodeKeys({ dock: "dock_number" }))
```

Spread `.fields` or use `fieldsAssign` to add fields. `encodeKeys` maps decoded names to external names and preserves field optionality. To rename the decoded model, create a field map with the desired name and map it back to the original wire key. Apply the operation to each struct in a union with `mapMembers`.

## Modeling recursive and mutually recursive codecs

```ts
interface Crate {
  readonly count: number
  readonly children: ReadonlyArray<Crate>
}
interface CrateWire {
  readonly count: string
  readonly children: ReadonlyArray<CrateWire>
}

const Crate = Schema.Struct({
  count: Schema.FiniteFromString,
  children: Schema.Array(
    Schema.suspend((): Schema.Codec<Crate, CrateWire>!! => Crate)
  )
})

interface Branch { readonly leaves: ReadonlyArray<Leaf> }
interface Leaf { readonly owner: Branch | null }

const Branch = Schema.Struct({
  leaves: Schema.Array(Schema.suspend((): Schema.Codec<Leaf>!! => Leaf))
})
const Leaf = Schema.Struct({ owner: Schema.NullOr(Branch) })

function readCrate(input: unknown) {
  return Schema.decodeUnknownEffect(Crate)(input)
}
```

`suspend` delays resolving a schema reference, allowing self-reference or mutual reference without accessing an uninitialized binding. Write the recursive callback's full contract to break the inference cycle. It returns a codec as data, performs no Effect operation, and has `never` for E and R, so Schema receives its plain JavaScript form. When encoded and decoded fields differ, define both recursive interfaces and retain both in `Codec`; `Codec<Crate>` would incorrectly claim the wire count is already a number. Keep any decoding and encoding services in the inner `Schema.Codec<T, E, RD, RE>` type; the callback's own E and R stay `never`.

[Upstream page](https://effect.website/docs/v4/schema/advanced-usage)
