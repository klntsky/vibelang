# Basic Usage

Schema constructors work in VibeLang as they do in Effect. Combine small descriptions into arrays, objects, and alternatives, then run a parser inside a function. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Modeling primitives, symbols, literals, enums, and template strings

```vibe
const Measurement = Schema.Finite
const UnrestrictedNumber = Schema.Number
const Enabled = Schema.Boolean
const Counter = Schema.BigInt
const Payload = Schema.Unknown
const Marker = Symbol.for("shipping/marker")
const MarkerSchema = Schema.UniqueSymbol(Marker)
const State = Schema.Literals(["queued", "packed", "sent"])
const PendingState = State.pick(["queued", "packed"])
const TrackingCode = Schema.TemplateLiteral(["pkg_", Schema.String])

enum Priority { Normal = "normal", Urgent = "urgent" }
const PrioritySchema = Schema.Enum(Priority)
```

`Schema.String`, `Boolean`, `BigInt`, and `Symbol` describe their JavaScript primitives. `Finite` rejects `NaN` and infinities; `Number` permits them. `ObjectKeyword` describes objects, `Undefined` and `Void` describe undefined values, `Null` describes null, `Unknown` accepts anything safely, `Any` opts out of type safety, and `Never` accepts nothing.

`Literal(value)` accepts one string, number, boolean, or bigint; `Literals([...])` combines exact values. Use `Schema.Null` for null. Literal collections expose `.literals`, enum schemas expose `.enums`, and `revealCodec(schema)` reveals the full codec type without changing the value. `UniqueSymbol` checks the particular symbol, rather than any symbol.

```vibe
const BinPosition = Schema.TemplateLiteralParser([
  "bin:", Schema.FiniteFromString, "/", Schema.NonEmptyString
])

function locateBin(input: unknown) {
  return Schema.decodeUnknownEffect(BinPosition)(input)
}
// "bin:12/upper" becomes ["bin:", 12, "/", "upper"]
```

`TemplateLiteral` validates a string pattern using literal, string, finite-number, union, or branded spans. `TemplateLiteralParser` also splits it into a tuple and applies the spans' conversions. Encoding that tuple reconstructs the string.

## Building unions and tagged alternatives

```vibe
const Warehouse = Schema.TaggedStruct("Warehouse", { aisle: Schema.String })
const Courier = Schema.TaggedStruct("Courier", { route: Schema.Int })
const Location = Schema.Union([Warehouse, Courier])
const TextOrCount = Schema.Union([Schema.String, Schema.Finite])
const NullableText = Schema.NullOr(Schema.String)
const MaybeText = Schema.UndefinedOr(Schema.String)
const NullishText = Schema.NullishOr(Schema.String)

function describeLocation(input: unknown) {
  const location = Schema.decodeUnknownEffect(Location)(input)
  return location._tag === "Warehouse" ? location.aisle : String(location.route)
}
```

A union accepts one of its members and exposes them through `.members`. With overlapping alternatives, the first successful member wins; put the more specific object first to avoid stripping fields through a broader match. Literal discriminator fields make alternatives unambiguous and enable ordinary TypeScript narrowing.

To attach a tag that is absent from the wire representation, add a field with `Schema.tagDefaultOmit("Warehouse")`. It supplies the tag during decoding and omits it during encoding. `NullOr`, `UndefinedOr`, and `NullishOr` accept values in addition to the wrapped schema; they do not make an object key optional.

## Defining tuples with optional and rest elements

```vibe
const Coordinate = Schema.Tuple([
  Schema.Finite.annotateKey({ title: "Longitude" }),
  Schema.Finite,
  Schema.optionalKey(Schema.String)
])
const Extended = Schema.Tuple([...Coordinate.elements, Schema.optionalKey(Schema.Boolean)])
const Readings = Schema.TupleWithRest(
  Schema.Tuple([Schema.String, Schema.optionalKey(Schema.Finite)]),
  [Schema.Boolean]
)
const BoundedReadings = Schema.TupleWithRest(
  Schema.Tuple([Schema.String]),
  [Schema.Finite, Schema.Boolean]
)
```

Tuples are readonly and position-sensitive. Optional elements use `optionalKey`; a present element must still match its schema. `TupleWithRest` takes a fixed tuple and a rest list whose first schema repeats; later schemas are required trailing elements. `Readings` accepts a label, an optional finite reading, then booleans. It can end after the label, but adding booleans requires the reading in its fixed position: `["sensor", 2, true]` is valid and `["sensor", true]` is not. `BoundedReadings` ends with a boolean after any repeated numbers.

Access fixed elements with `.elements`, or with `.schema.elements` on a tuple with rest. Its `.rest` describes the repeating and trailing schemas. Element annotations describe individual positions.

## Modeling mutable and nonempty arrays

```vibe
const Labels = Schema.Array(Schema.String)
const EditableLabels = Schema.mutable(Labels)
const RequiredLabels = Schema.NonEmptyArray(Schema.NonEmptyString)

function addLabel(input: unknown) {
  const labels = Schema.decodeUnknownEffect(EditableLabels)(input)
  labels.push("inspected")
  return labels
}
```

`Array` accepts zero or more elements; `NonEmptyArray` requires at least one. Both expose the element schema as `.value`. `mutable` changes the array's mutability at the outer level and does not make nested structs mutable.

## Declaring records and index signatures

```vibe
const Stock = Schema.Record(Schema.String, Schema.Int)
const SymbolStock = Schema.Record(Schema.Symbol, Schema.Int)
const RegionalStock = Schema.Record(Schema.Literals(["east", "west"]), Schema.Int)
const NamedCounters = Schema.Record(
  Schema.TemplateLiteral(["count_", Schema.String]), Schema.Int
)
const NormalizedStock = Schema.Record(
  Schema.String.pipe(Schema.decodeTo(Schema.String, SchemaTransformation.toLowerCase())),
  Schema.Int
)
const LabeledCounters = Schema.StructWithRest(
  Schema.Struct({ total: Schema.Int }),
  [Schema.Record(Schema.String, Schema.Int)]
)
```

Records describe string or symbol keys and a value schema. A finite literal key set describes required properties; template literals and string checks restrict an open key space. A key that fails a record's key check is normally discarded as excess; pass `onExcessProperty: "error"` to reject it.

Record key codecs can transform selected keys. `NormalizedStock` decodes `{ NORTH: 2 }` to `{ north: 2 }`. Selection uses encoded property names before transforming them. With sequential parsing, when transformed keys collide, the later selected property overwrites the earlier value. Use a whole-object transformation when you need a different collision policy.

`StructWithRest` combines fixed fields with index signatures, including several signatures for the same key kind. Keep fixed-field types compatible with the index value type; otherwise split the fixed and indexed data into separate structures.

Use `.key` and `.value` to inspect a record; a struct with rest exposes `.schema.fields` and `.records`. Use `Schema.mutableKey` on the value schema for mutable record entries.

## Creating structs and tagged structs

```vibe
const Package = Schema.Struct({
  reference: Schema.String,
  weight: Schema.Finite,
  comment: Schema.optionalKey(Schema.String)
})
const Packed = Schema.TaggedStruct("Packed", Package.fields)
const Marked = Schema.Struct({
  _tag: Schema.tag("Marked"),
  category: Schema.tag("fragile"),
  reference: Schema.String
})

function createPacked() {
  return Packed.makeEffect({ reference: "box-76", weight: 2 })
}
```

Structs describe fixed fields and expose them through `.fields`. `Struct({})` accepts any non-nullish value, so use a more specific description when an actual object shape matters. Change a field's mutability with `mutableKey`.

`tag` supplies a constructor default, while `TaggedStruct` adds a `_tag` field for you. Construction can omit the tag, but ordinary decoding requires it in the input. Use `tagDefaultOmit` when decoding should supply it too.

## Validating class instances

```vibe
class Pallet {
  constructor(readonly code: string) {}
}

const PalletSchema = Schema.instanceOf(Pallet).check(
  Schema.makeFilter((pallet) => pallet.code.length > 0 || "Pallet code is empty")
)

function readPallet(input: unknown) {
  return Schema.decodeUnknownEffect(PalletSchema)(input)
}
```

`instanceOf` checks class identity; an object with a matching `code` field is not an instance. Add checks to validate fields after identity succeeds. Classes with private constructors need `Schema.declare` with a type guard instead of `instanceOf`.

## Selecting, removing, and changing optional fields

```vibe
const PackageSummary = Package.mapFields(Struct.pick(["reference", "weight"]))
const WithoutComment = Package.mapFields(Struct.omit(["comment"]))
const Patch = Package.mapFields(Struct.map(Schema.optionalKey))
const LoosePatch = Package.mapFields(Struct.map(Schema.optional))
const RequiredPatch = Patch.mapFields(Struct.map(Schema.requiredKey))
const Editable = Package.mapFields(Struct.map(Schema.mutableKey))
```

`mapFields` builds a new struct from transformed field descriptions. `optionalKey` permits omission while `optional` also accepts explicit `undefined`. `requiredKey` removes key optionality but does not remove an existing `undefined` union. Field transformations do not automatically preserve whole-struct checks, so reapply checks that still describe the new shape.

[Upstream page](https://effect.website/docs/v4/schema/basic-usage)
