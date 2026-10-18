# Default Constructors

Schema constructors work as in Effect and operate on decoded values. They apply constructor defaults and type-side checks without running the codec's encoded-to-decoded conversion. In local Effect 4.0.5, `makeEffect` fails with a raw schema issue. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Constructing structs and records

```vibe
const Label = Schema.Struct({ text: Schema.NonEmptyString })
const LabelsById = Schema.Record(Schema.String, Schema.NonEmptyString)

function makeLabels() {
  const label = Label.makeEffect({ text: "Keep upright" })
  const byId = LabelsById.makeEffect({ handling: label.text })
  return { label, byId }
}
```

Every schema exposes `make`, `makeOption`, and `makeEffect`. A constructor takes the schema's constructor input and returns its decoded type. `FiniteFromString.makeEffect(6)` constructs a number; its decoder instead accepts text. Struct and record constructors validate fields and entries.

## Applying filters and brands during construction

```vibe
const CaseCount = Schema.FiniteFromString.check(
  Schema.isInt(), Schema.isBetween({ minimum: 1, maximum: 48 })
).pipe(Schema.brand("CaseCount"))

function makeCaseCount(value: number) {
  return CaseCount.makeEffect(value)
}
```

Filters run during construction as well as parsing. A filter leaves the static type unchanged; a brand returns a branded number while keeping the runtime number representation. `{ disableChecks: true }` deliberately skips checks, but does not decode an encoded input or establish that the value is valid.

## Handling constructor failures

```vibe
function constructOrExplain(value: number) {
  try {
    return String(CaseCount.makeEffect(value))
  } catch (issue) {
    return try((issue) => SchemaIssue.makeFormatterDefault()(issue), issue)
  }
}

function constructAsOption(value: number) {
  return try((value) => CaseCount.makeOption(value), value)
}

function constructWithThrowingApi(value: number) {
  return try((value) => CaseCount.make(value), value)
}
```

`makeEffect` fails with `SchemaIssue.Issue`, not the `SchemaError` wrapper returned by Schema decoders. Format the issue directly or translate it to an application error. `makeOption` discards schema-validation details, while `make` throws a generic validation Error with the issue in `cause`. Both synchronous APIs can throw for non-schema causes or asynchronous work, so their explicit boundaries remain useful.

## Setting nested and lazy defaults

```vibe
const Packing = Schema.Struct({
  handling: Schema.String.pipe(Schema.withConstructorDefault(() => "standard")),
  dimensions: Schema.Struct({
    unit: Schema.String.pipe(Schema.withConstructorDefault(() => "cm")),
    width: Schema.Finite
  }).pipe(Schema.withConstructorDefault(() => ({ width: 10 }))),
  labels: Schema.Array(Schema.String).pipe(Schema.withConstructorDefault(() => []))
})

function makePacking() {
  return Packing.makeEffect({})
}
// { handling: "standard", dimensions: { unit: "cm", width: 10 }, labels: [] }
```

Pass a program to `withConstructorDefault`; the constructor runs it only when the field needs a default. An outer default supplies nested constructor input, after which inner defaults fill their own fields. A fresh `[]` is created each time the labels default runs, rather than sharing one mutable array.

The default program may fail with schema issues but cannot require services. `make` and `makeOption` need defaults that complete synchronously; use `makeEffect` if a default pauses. Read service-dependent values in the enclosing program and pass them as explicit fields instead.

## Reusing defaults

```vibe
const ReturnPacking = Schema.Struct({
  reason: Schema.String,
  handling: Packing.fields.handling,
  labels: Packing.fields.labels
})

function makeReturnPacking() {
  return ReturnPacking.makeEffect({ reason: "inspection" })
}
```

Defaults belong to field descriptions. Reusing a field carries its constructor default into the new struct. A documented `default` annotation is metadata only, and a constructor default does not make decoding accept a missing field; add a decoding default separately when both behaviors are required.

## Initializing schema-backed classes

```vibe
class PackingRequest extends Schema.Class<PackingRequest>("PackingRequest")({
  reference: Schema.String,
  handling: Packing.fields.handling,
  labels: Packing.fields.labels
}) {}

function createPackingRequest(reference: string) {
  return PackingRequest.makeEffect({ reference })
}
```

Classes use the same default-bearing field descriptions as structs. The result is a validated instance with its class behavior. `new PackingRequest(...)` also applies defaults. Its exceptions are defects unless an explicit `try` hatch gives them an expected error type; `new` itself never runs a program.

[Upstream page](https://effect.website/docs/v4/schema/default-constructors)
