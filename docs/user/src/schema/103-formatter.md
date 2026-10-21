# Schema to Formatter

Value formatters work exactly as in Effect. A schema-derived formatter renders decoded values as readable strings for diagnostics or display. It neither validates input nor serializes for transport; use a codec for serialization and an issue formatter for failures. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Deriving value formatters from schemas

```vibe
// run: Console.log(displayDispatch({ depot: "east", counts: [2, 4] }))
import { Console } from "effect"
const Dispatch = Schema.Struct({
  depot: Schema.String,
  counts: Schema.Array(Schema.Int)
})

function displayDispatch(input: unknown) {
  const dispatch = Schema.decodeUnknownEffect(Dispatch)(input)
  return try((schema, dispatch) => Schema.toFormatter(schema)(dispatch), Dispatch, dispatch)
}
// For { depot: "east", counts: [2, 4] }, produces a readable object representation.
```

```output
{ "depot": "east", "counts": [2, 4] }
```

`toFormatter` walks structs, arrays, unions, and supported declarations. It takes the schema's decoded type, even when the codec accepts a different wire representation, so the example decodes first to give it valid data. The explicit boundary accounts for exceptions from generated or custom formatting callbacks.

## Customizing formatting at selected syntax-tree nodes

```vibe
// run: Console.log(displayManifest({ label: "crate", weight: 2 }))
const Weight = Schema.Finite.pipe(Schema.overrideToFormatter(
  () => (value) => `${value} kg`
))
schema Manifest { label: string; weight: Weight }

function displayManifest(input: unknown) {
  const manifest = Schema.decodeUnknownEffect(Manifest)(input)
  return try((schema, manifest) => Schema.toFormatter(schema, {
    onBefore: (ast) => ast._tag === "String"
      ? (value: string) => `[${value}]`
      : undefined
  })(manifest), Manifest, manifest)
}
```

```output
{ "label": [crate], "weight": 2 kg }
```

`overrideToFormatter` replaces formatting at one schema node, here the weight. A `toFormatter` annotation takes priority; `onBefore` then intercepts selected AST nodes before built-in derivation, and returning undefined keeps the default.

An opaque declaration can supply `toFormatter` when declared; a parameterized one receives the formatter for each parameter. Keep hooks and generated formatter functions synchronous and pure; compute service-dependent display data before formatting. Check custom output for exposed secrets before logging it.

[Upstream page](https://effect.website/docs/v4/schema/formatter)
