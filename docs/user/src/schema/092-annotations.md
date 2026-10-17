# Schema Annotations

Annotations work in Effeck exactly as in Effect. They attach metadata or interpreter hooks without changing the value's static type. Use them for documentation, error messages, representation choices, and custom tooling. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Attaching built-in metadata

```ts
const Destination = Schema.String.annotate({
  message: "Destination must be text"
}).check(Schema.isNonEmpty({ message: "Choose a destination" })).annotate({
  identifier: "Destination",
  title: "Shipping destination",
  description: "The destination printed on the manifest",
  documentation: "Keep destination names consistent across depots.",
  examples: ["North depot"],
  default: "Central depot"
})

const Request = Schema.Struct({
  destination: Destination.annotateKey({ messageMissingKey: "Destination is required" })
})
```

`identifier` gives interpreters a stable name; `title`, `description`, and `documentation` explain the value. `examples` and `default` document values without checking examples or applying defaults. Use explicit constructor or decoding default combinators to change runtime behavior.

`message` targets a schema node or check; annotating after a check targets the last check. `annotateKey` applies to a struct field or tuple position, and `annotateEncoded` targets a codec's external representation. `messageUnexpectedKey` customizes excess-key errors.

Representation hooks include `toCodec`, `toCodecJson`, `toCodecStringTree`, `toCodecIso`, and `toCodecArbitrary`. `toFormatter` and `toEquivalence` define behavior for opaque declarations; checks use `toJsonSchema` and `arbitraryConstraint` to explain their constraints to interpreters. Pass parse options to a parser rather than relying on arbitrary annotation keys to change execution.

## Controlling decoding concurrency

```ts
const ReviewedLabel = Schema.String.pipe(Schema.decode({
  decode: SchemaGetter.checkEffect((label) => {
    sleep("10 millis")
    return label.length > 0 || "Empty label"
  }),
  encode: SchemaGetter.passthrough()
}))
const Labels = Schema.Array(ReviewedLabel)

function reviewLabels(input: unknown) {
  return Schema.decodeUnknownEffect(Labels)(input, { concurrency: 3 })
}
```

Product schemas such as arrays, structs, and tuples can run effectful child parsers concurrently. The default is sequential; use a positive concurrency limit or `"unbounded"`. Concurrency changes completion order of side effects, so do not make a check depend on which sibling finishes first. Pass the option when creating or applying the parser.

## Recovering decoding failures with fallback values

```ts
const OptionalLabel = Schema.String.pipe(
  Schema.catchDecoding((issue) => {
    log(`label fallback: ${issue._tag}`)
    return Option.some("unlabeled")
  })
)

function readLabel(input: unknown) {
  return Schema.decodeUnknownEffect(OptionalLabel)(input)
}
```

`catchDecoding` receives the decoding issue and returns an Effect producing an `Option` of the decoded value. `Some` provides a fallback value; `None` represents absence, which only succeeds where the surrounding schema permits it. Throw the issue to propagate it. A fallback changes decoding only; it does not automatically repair encoding or capture defects.

Log the issue tag or stage when diagnostics help. Avoid recording the rejected payload when it contains secrets or customer data.

## Defining custom schema annotations

```ts
const InternalOnly = Symbol.for("warehouse/schema/internal-only")

declare module "effect/Schema" {
  namespace Annotations {
    interface Annotations {
      [InternalOnly]?: boolean
    }
  }
}

const DepotCode = Schema.String.annotate({ [InternalOnly]: true })

function isInternal(schema: Schema.Top) {
  return Schema.resolveAnnotations(schema)?.[InternalOnly] ?? false
}
```

Use a namespaced symbol to avoid collisions with other tools. Module augmentation makes the annotation's value type checkable. `resolveAnnotations` retrieves metadata from the appropriate schema node; application tooling must explicitly interpret custom metadata for it to have an effect.

[Upstream page](https://effect.website/docs/v4/schema/annotations)
