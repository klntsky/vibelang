# Schema Projections

Projections work in Effeck exactly as in Effect. They turn one side of a codec into a schema that validates that representation without running the codec's conversion. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Projecting a codec to its decoded schema

```ts
const CountFromText = Schema.FiniteFromString.check(
  Schema.isInt(), Schema.isGreaterThanOrEqualTo(1)
)
const CountValue = Schema.toType(CountFromText)

function inspectCount(input: unknown) {
  return Schema.decodeUnknownEffect(CountValue)(input)
}
// Accepts 4. Rejects "4", 0, and 1.5.
```

`toType` makes the original decoded type both the new decoded and encoded type. It keeps the checks on that side and drops conversion steps and service requirements. Use it when a value has already been decoded, or when the first stage of a composed codec has already converted nested elements.

## Projecting a codec to its encoded schema

```ts
const Name = Schema.String.check(Schema.isMinLength(3)).pipe(
  Schema.decodeTo(Schema.Trim)
)
const NameInput = Schema.toEncoded(Name)

function inspectWireName(input: unknown) {
  return Schema.decodeUnknownEffect(NameInput)(input)
}
// " x " passes unchanged: it has length 3 before trimming.
```

`toEncoded` uses the original encoded type on both sides. It validates the input representation without trimming it and requires no conversion services. Use this projection to inspect wire data or to generate examples of encoded values.

## Retaining checks appropriate to each side

```ts
const ParcelCount = Schema.String.check(Schema.isMinLength(2)).pipe(
  Schema.decodeTo(Schema.FiniteFromString),
  Schema.check(Schema.isGreaterThan(5))
)
const TextOnly = Schema.toEncoded(ParcelCount)
const NumberOnly = Schema.toType(ParcelCount)

function compareViews() {
  const text = Schema.decodeUnknownEffect(TextOnly)("00") // "00"
  const count = Schema.decodeUnknownEffect(NumberOnly)(8) // 8
  return { text, count }
}
```

`TextOnly` keeps the two-character input check and does not enforce the numeric bound. `NumberOnly` keeps the bound above five and has no string-length requirement. Neither projection promises that a value can complete the original full conversion: `"00"` satisfies the encoded projection but decodes to a number rejected by the original codec.

[Upstream page](https://effect.website/docs/v4/schema/projections)
