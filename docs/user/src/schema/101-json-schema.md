# Schema to JSON Schema

JSON Schema export works in EffectScript as in Effect. It describes the encoded side of a schema's canonical JSON codec, so the exported shape matches the selected JSON representation. Export is best-effort and does not replace runtime decoding. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Exporting JSON Schema documents

```ts
const Depot = Schema.Struct({
  name: Schema.String,
  capacity: Schema.Finite
})

function depotDocument() {
  return Effect.try(() => Schema.toJsonSchemaDocument(Depot))
}
```

The returned document contains `dialect`, the root `schema`, and a separate `definitions` map. The default dialect is `"draft-2020-12"`. The default excess-property policy leaves unmodeled keys open even though the default runtime decoder removes them.

## Selecting canonical representations and specification drafts

```ts
import { JsonSchema } from "effect"

function alternateDrafts() {
  const document = Effect.try(() => Schema.toJsonSchemaDocument(
    Schema.Tuple([Schema.String, Schema.FiniteFromString])
  ))
  const draft07 = Effect.try(() => JsonSchema.toDocumentDraft07(document))
  const draft04 = Effect.try(() => JsonSchema.toDocumentDraft04(document))
  return { draft07, draft04 }
}
```

Export first derives `toCodecJson`, then describes its encoded side. `FiniteFromString` therefore exports a string schema, while a runtime Date uses its canonical JSON representation. Custom declaration hooks define the representation used by both runtime JSON codecs and export. Convert the resulting document through `JsonSchema` when another draft is required.

## Carrying annotations and encoded-side metadata

```ts
const DepotName = Schema.Trim.pipe(Schema.annotateEncoded({
  title: "Depot input",
  description: "Text entered before surrounding whitespace is removed",
  examples: [" north "],
  default: "central",
  "x-widget": "depot-picker"
}))

function nameDocument() {
  return Effect.try(() => Schema.toJsonSchemaDocument(DepotName, {
    includeAnnotationKey: (key) => key.startsWith("x-")
  }))
}
```

Use `annotateEncoded` for metadata attached to the wire representation; `.annotate` targets the decoded side. Standard JSON Schema annotations include `title`, `description`, `default`, `examples`, `readOnly`, `writeOnly`, `format`, `contentEncoding`, `contentMediaType`, and `contentSchema`. `includeAnnotationKey` admits selected nonstandard keys without removing standard ones. A documented default does not make the runtime decoder apply it.

## Exporting constraints and optional fields

```ts
const DepotRequest = Schema.Struct({
  code: Schema.String.check(Schema.isPattern(/^[A-Z]{2}[0-9]{3}$/)),
  note: Schema.optionalKey(Schema.String),
  alias: Schema.optional(Schema.String),
  capacity: Schema.Int.check(Schema.isBetween({ minimum: 1, maximum: 120 }))
})

function requestDocument() {
  return Effect.try(() => Schema.toJsonSchemaDocument(DepotRequest))
}
```

Built-in filters contribute constraints such as numeric bounds, lengths, patterns, and uniqueness. An exact optional key is omitted from `required`; `optional` also allows undefined, which the canonical JSON representation expresses as null. Custom filters need a `toJsonSchema` annotation to describe a representable constraint.

String lengths have a representational difference: JavaScript counts UTF-16 code units while JSON Schema counts Unicode code points. Built-in export uses conservative bounds where needed; `isMinLength(3)` can become `minLength: 2`. Arbitrary predicates and opaque declarations without a structural JSON codec cannot always be described exactly.

## Emitting references for recursive definitions

```ts
interface DepotTree {
  readonly name: string
  readonly branches: ReadonlyArray<DepotTree>
}
const DepotTree = Schema.Struct({
  name: Schema.String,
  branches: Schema.Array(Schema.suspend((): Effect<Schema.Codec<DepotTree>, never, never> => DepotTree))
}).annotate({ identifier: "DepotTree" })

function treeDocument() {
  return Effect.try(() => Schema.toJsonSchemaDocument(DepotTree))
}
```

An identifier creates a named definition referenced through `$ref`. Recursive export needs an identifier so the cycle can be represented as a reference rather than an infinite expansion. Reuse stable identifiers for shared descriptions and avoid assigning the same identifier to different shapes.

## Adjusting document generation

```ts
function strictDocument() {
  return Effect.try(() => Schema.toJsonSchemaDocument(DepotRequest, {
    onExcessProperty: "error",
    generateDescriptions: true,
    includeAnnotationKey: (key) => key.startsWith("x-")
  }))
}
```

`onExcessProperty: "error"` closes unmodeled keys where their key space can be expressed. Match it with the same runtime parsing option when exported validation should reject those keys too. `generateDescriptions` derives missing check descriptions from `expected`; `referencePolicy` controls allocation of named references and receives canonical JSON ASTs. `includeAnnotationKey` controls additional metadata.

## Describing JSON string codecs

```ts
const DepotText = Schema.fromJsonString(Depot)

function jsonTextDocument() {
  return Effect.try(() => Schema.toJsonSchemaDocument(DepotText))
}
```

The external value is a string containing JSON, so the root schema is a string with `contentMediaType: "application/json"`. Its content metadata describes the embedded representation. This differs from exporting `Depot` itself, whose external JSON value is an object. Content keywords document the embedded format; validator support for enforcing them varies.

[Upstream page](https://effect.website/docs/v4/schema/json-schema)
