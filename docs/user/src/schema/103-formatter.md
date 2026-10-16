# Schema to Formatter

Value formatters work in EffectScript exactly as in Effect. A schema-derived formatter renders decoded values as readable strings for diagnostics or display. It does not validate input or serialize a value for transport; use a codec for serialization and an issue formatter for failures. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Deriving value formatters from schemas

```ts
const Dispatch = Schema.Struct({
  depot: Schema.String,
  counts: Schema.Array(Schema.Int)
})

function displayDispatch(input: unknown) {
  const dispatch = Schema.decodeUnknownEffect(Dispatch)(input)
  return Effect.try(() => Schema.toFormatter(Dispatch)(dispatch))
}
// For { depot: "east", counts: [2, 4] }, produces a readable object representation.
```

`toFormatter` walks structs, arrays, unions, and supported declarations. Its argument type is the schema's decoded type, even when the codec accepts a different wire representation. The example decodes first so the formatter receives valid data. The explicit boundary accounts for exceptions from generated or custom formatting callbacks.

## Customizing formatting at selected syntax-tree nodes

```ts
const Weight = Schema.Finite.pipe(Schema.overrideToFormatter(
  () => (value) => `${value} kg`
))
const Manifest = Schema.Struct({ label: Schema.String, weight: Weight })

function displayManifest(input: unknown) {
  const manifest = Schema.decodeUnknownEffect(Manifest)(input)
  return Effect.try(() => Schema.toFormatter(Manifest, {
    onBefore: (ast) => ast._tag === "String"
      ? (value: string) => `[${value}]`
      : undefined
  })(manifest))
}
```

`overrideToFormatter` replaces formatting at one schema node, such as the weight above. `onBefore` intercepts selected AST nodes before normal derivation; returning undefined keeps the default behavior. The example changes string rendering while retaining the weight formatter.

Opaque declarations can supply `toFormatter` when declared. A parameterized declaration receives the formatter for each parameter. Keep hooks and generated formatter functions synchronous and pure; compute service-dependent display data before formatting. Review custom output for secret exposure before using it in logs.

[Upstream page](https://effect.website/docs/v4/schema/formatter)
