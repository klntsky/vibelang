# Canonical Codecs

Canonical codec derivation works as in Effect: it preserves a schema's decoded type and chooses a standard external representation for JSON, string trees, or structural updates. Derivation only builds a codec; parsing and encoding are separate calls. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Deriving JSON, string-tree, and isomorphism codecs

```vibe
// run: Console.log(JSON.stringify(encodeAudit({ created: new Date("2026-01-02T00:00:00Z"), sequence: 12n, labels: ["fragile"] })))
import { Console } from "effect"
const AuditRecord = Schema.Struct({
  created: Schema.Date,
  sequence: Schema.BigInt,
  labels: Schema.Array(Schema.String)
})

function encodeAudit(input: unknown) {
  const value = Schema.decodeUnknownEffect(AuditRecord)(input)
  const jsonCodec = Schema.toCodecJson(AuditRecord)
  return Schema.encodeEffect(jsonCodec)(value)
}
// The JSON representation contains date and bigint strings.
```

```output
{"created":"2026-01-02T00:00:00.000Z","sequence":"12","labels":["fragile"]}
```

`toCodecJson` recursively chooses JSON-compatible encodings for domain values, nested declarations with built-in representations included. It keeps explicit encodings: a date explicitly encoded as epoch milliseconds stays numeric. Its output is a JSON value, not serialized JSON text; for text, use `fromJsonString`.

```vibe
// run: Console.log(JSON.stringify(encodeSearch()))
const Search = Schema.Struct({
  offset: Schema.Int,
  archived: Schema.Boolean,
  labels: Schema.Array(Schema.String)
})

function encodeSearch() {
  const codec = Schema.toCodecStringTree(Search)
  return Schema.encodeEffect(codec)({ offset: 20, archived: false, labels: ["large"] })
}
// { offset: "20", archived: "false", labels: ["large"] }
```

```output
{"offset":"20","archived":"false","labels":["large"]}
```

`toCodecStringTree` preserves object and array structure and converts leaves to strings. A string tree can also contain `undefined`, so it is not limited to JSON values. It suits configuration, forms, and query parameters.

```vibe
// run: const slot = slotRepresentation({ row: 2, column: 3 }); Console.log(slot instanceof Slot, slot.row, slot.column)
class Slot extends Schema.Class<Slot>("Slot")({
  row: Schema.Int,
  column: Schema.Int
}) {}

function slotRepresentation(input: unknown) {
  const slot = Schema.decodeUnknownEffect(Slot)(input)
  const codec = Schema.toCodecIso(Slot)
  const structural = Schema.encodeEffect(codec)(slot)
  return Schema.decodeUnknownEffect(codec)(structural)
}
```

```output
true 2 3
```

`toCodecIso` selects the schema's `Iso` representation, such as plain fields for a class, for tools that update plain objects and arrays, then reconstruct the domain value. It is a structural isomorphism codec: it does not guarantee that the original runtime object is retained by identity.

## Supplying canonical conversions for custom declarations

```vibe
// run: Console.log(JSON.stringify(encodeDepot()))
class DepotRef {
  constructor(readonly region: string, readonly number: number) {}
}

const DepotRefSchema = Schema.instanceOf(DepotRef, {
  toCodecJson: () => Schema.link<DepotRef>()(
    Schema.Struct({ region: Schema.String, number: Schema.Int }),
    SchemaTransformation.transform({
      decode: (data) => new DepotRef(data.region, data.number),
      encode: (ref) => ({ region: ref.region, number: ref.number })
    })
  )
})

function encodeDepot() {
  const codec = Schema.toCodecStringTree(DepotRefSchema)
  return Schema.encodeEffect(codec)(new DepotRef("nw", 14))
}
// { region: "nw", number: "14" }
```

```output
{"region":"nw","number":"14"}
```

Declarations are opaque, so give derivation a structural codec. JSON prefers `toCodecJson`, then `toCodec`; StringTree prefers `toCodecStringTree`, then `toCodecJson`, then `toCodec`; Iso prefers `toCodecIso`, then `toCodec`. A JSON or StringTree hook can return `undefined` to mark the declaration as already canonical.

Without a hook, JSON uses an unconstrained JSON fallback and parsing can still fail; StringTree derivation throws; Iso keeps the declaration's own representation. Hooks can use effectful conversions but cannot add services absent from the resulting codec's service type. The separate `Schema.toIso` helper consumes its Iso conversion synchronously.

## Relating canonical representations to JSON Schema output

```vibe
// run: Console.log(JSON.stringify(describeDepotJson()))
function describeDepotJson() {
  return Schema.toJsonSchemaDocument(DepotRefSchema)
}
```

```output
{"dialect":"draft-2020-12","schema":{"type":"object","properties":{"region":{"type":"string"},"number":{"type":"integer","minimum":-9007199254740991,"maximum":9007199254740991}},"required":["region","number"],"additionalProperties":true},"definitions":{}}
```

JSON Schema export derives the canonical JSON codec first and describes its encoded side, so the custom depot representation appears as an object with a region and number. A custom JSON hook controls both runtime serialization and the exported shape; an opaque declaration without a structural hook yields an unconstrained document.

[Upstream page](https://effect.website/docs/v4/schema/canonical-codecs)
