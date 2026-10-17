# Schema Transformations

Transformations work in Effeck as in Effect: a codec validates one representation, converts it, and validates the result. Use `decodeTo` to connect schemas, then supply pure conversions or effectful getters as needed. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Defining reversible infallible conversions

```ts
const Availability = Schema.Literals(["ready", "held"]).pipe(
  Schema.decodeTo(Schema.Boolean, SchemaTransformation.transform({
    decode: (status) => status === "ready",
    encode: (available) => available ? "ready" : "held"
  }))
)

function readAvailability(input: unknown) {
  return Schema.decodeUnknownEffect(Availability)(input)
}
```

`SchemaTransformation.transform` takes two synchronous, infallible functions. Input validation can still fail before either function runs, and output validation can fail afterward. Pure callbacks cannot call services or throw typed failures; choose effectful getters for those operations.

## Understanding transformation inputs and outputs

```ts
const NumericAvailability = Schema.FiniteFromString.pipe(
  Schema.decodeTo(Availability, SchemaTransformation.transform({
    decode: (quantity) => quantity > 0 ? "ready" : "held",
    encode: (status) => status === "ready" ? 1 : 0
  }))
)

function readNumericAvailability() {
  return Schema.decodeEffect(NumericAvailability)("12") // true
}
```

Decoding first runs the source codec, then converts the source's `Type` to the target's `Encoded`, then runs the target codec. Encoding performs the reverse sequence. The complete codec has the source's `Encoded` and the target's `Type`.

The middle functions above handle a number and a status string, even though the complete codec connects strings to booleans. This conversion normalizes quantities: encoding `true` produces `"1"`, so the original `"12"` is not preserved.

## Defining fallible conversions

```ts
const DeliveryMode = Schema.String.pipe(
  Schema.decodeTo(Schema.Literals(["road", "rail"]), {
    decode: SchemaGetter.transformEffect<"road" | "rail", string>((text) => {
      const mode = text.trim().toLowerCase()
      if (mode === "road" || mode === "rail") return mode
      throw new SchemaIssue.InvalidValue({ message: "Choose road or rail" })
    }),
    encode: SchemaGetter.passthrough()
  })
)
```

`transformEffect` receives the converted input and active parse options. Its callback returns a value directly in Effeck and throws a `SchemaIssue.Issue` for an expected conversion failure. The outer Schema parser wraps issues in `SchemaError`. A domain error from a service must be translated to an issue before it escapes the getter.

## Running asynchronous conversions with dependencies

```ts
error UnknownRoute {
  code: string
}

abstract class Routes {
  abstract check(code: string): void!UnknownRoute!
}

const RouteCode = Schema.String.pipe(
  Schema.decodeTo(Schema.NonEmptyString, {
    decode: SchemaGetter.transformEffect((code) => {
      try {
        Routes.check(code)
        return code
      } catch (error) {
        throw new SchemaIssue.InvalidValue({ message: "Route is unavailable" })
      }
    }),
    encode: SchemaGetter.passthrough()
  })
)

function readRoute(input: unknown) {
  return Schema.decodeUnknownEffect(RouteCode)(input)
}
// string!Schema.SchemaError!Routes

const TestRoutes = Layer.succeed(Routes, { check: (_code) => {} })

function previewRoute() {
  return Effect.provide(() => readRoute("coastal"), TestRoutes)
}
```

The getter can pause while its service runs. Decoding requires `Routes`; encoding does not, because its getter only passes through the value. Use the Effect interpreter whenever services or asynchronous work are involved. Plain `catch` translates expected service failures and leaves defects and interruption intact.

## Forbidding an encoding direction

```ts
const Initial = Schema.NonEmptyString.pipe(
  Schema.decodeTo(Schema.String, {
    decode: SchemaGetter.transform((name) => name[0]),
    encode: SchemaGetter.forbidden(() => "An initial cannot recover the full name")
  })
)

function readInitial(input: unknown) {
  return Schema.decodeUnknownEffect(Initial)(input)
}
```

A lossy conversion has no meaningful inverse. `forbidden` reports a schema issue when that direction runs, rather than pretending to reconstruct discarded information. The codec still has an encoding API, but using it fails.

## Composing transformations and effectful checks

```ts
const SemicolonNumbers = Schema.String.pipe(
  Schema.decodeTo(Schema.Array(Schema.String), SchemaTransformation.transform({
    decode: (text): ReadonlyArray<string>!! => text.split(";"),
    encode: (items) => items.join(";")
  })),
  Schema.decodeTo(Schema.Array(Schema.FiniteFromString))
)

const DeferredLabel = Schema.String.pipe(
  Schema.decode({
    decode: SchemaGetter.checkEffect((label) => {
      Effect.sleep("5 millis")
      return label.length > 2 || "Label is too short"
    }),
    encode: SchemaGetter.passthrough()
  })
)

function readNumbers() {
  return Schema.decodeEffect(SemicolonNumbers)("2;5;8")
}
```

Omit the custom conversion when the source's decoded type already fits the target's encoded type. `checkEffect` validates without changing the value and returns the same filter outputs as a synchronous predicate. It can pause or require services but cannot leak arbitrary domain errors; convert those into schema issues.

When converting a collection whose elements have already been decoded, use `Schema.toType(elementSchema)` on the target collection. Otherwise the element transformations run twice.

## Converting string case, JSON, and binary encodings

```ts
const LowerLabel = Schema.String.pipe(
  Schema.decodeTo(
    Schema.String.check(Schema.isLowercased()),
    SchemaTransformation.toLowerCase()
  )
)
const JsonLabel = Schema.fromJsonString(Schema.Struct({ label: Schema.String }))

function decodeTextFormats() {
  const trimmed = Schema.decodeEffect(Schema.Trim)("  crate  ")
  const lower = Schema.decodeEffect(LowerLabel)("CRATE")
  const data = Schema.decodeEffect(JsonLabel)('{"label":"crate"}')
  const base64 = Schema.decodeEffect(Schema.StringFromBase64)("Y3JhdGU=")
  const hex = Schema.decodeEffect(Schema.StringFromHex)("6372617465")
  return { trimmed, lower, data, base64, hex }
}
```

`Schema.Trim` removes surrounding whitespace. `SchemaTransformation.toUpperCase`, `capitalize`, and `uncapitalize` provide the other case conversions; pair each with an appropriate target check. Case conversion and trimming normalize values rather than preserving the original spelling.

`fromJsonString(schema)` parses JSON text and validates its contents; encoding serializes the encoded contents back to text. `StringFromBase64`, `StringFromBase64Url`, `StringFromHex`, and `StringFromUriComponent` decode encoded text to a UTF-8 string and encode it in the reverse direction. Compose the URI codec with a JSON-string codec for structured URL data.

## Parsing numeric, date, and decimal values

```ts
const Manifest = Schema.Struct({
  units: Schema.FiniteFromString,
  sequence: Schema.BigIntFromString,
  departure: Schema.DateFromString,
  price: Schema.BigDecimalFromString
})

function readManifest(input: unknown) {
  return Schema.decodeUnknownEffect(Manifest)(input)
}
```

`FiniteFromString` rejects unparseable and nonfinite numbers; it does not require an integer unless you add a check. `BigIntFromString` rejects fractions and invalid bigint text. `DateFromString` produces a valid `Date` and rejects invalid dates; encoding uses its normalized string representation. `BigDecimalFromString` preserves decimal precision without converting through a JavaScript number.

## Coming from Effect

Inside `SchemaGetter.transformEffect`, write ordinary statements and `throw` schema issues. Effeck turns the callback into the Effect expected by the getter, including its directional service requirements.

[Upstream page](https://effect.website/docs/v4/schema/transformations)
