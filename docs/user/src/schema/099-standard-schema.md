# Schema to Standard Schema

Standard Schema adaptation works as in Effect. It exposes a codec to libraries that implement the Standard Schema V1 validation interface while retaining the original Schema methods. The codec must have no decoding service requirements. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Adapting codecs to the standard validation interface

```ts
const Dispatch = Schema.Struct({
  reference: Schema.NonEmptyString,
  count: Schema.FiniteFromString
})
const DispatchStandard = Schema.toStandardSchemaV1(Dispatch, {
  parseOptions: { errors: "all" }
})

function standardMetadata() {
  return DispatchStandard["~standard"].vendor // "effect"
}
```

`toStandardSchemaV1` adds the `"~standard"` interface. Validation returns `{ value }` on success or `{ issues }` on failure, and performs decoding, including transformations. Pass the adapted schema to a compatible library from that library's normal TypeScript integration module. It still exposes the original Effect Schema APIs.

## Choosing synchronous or asynchronous validation

```ts
const ReviewedDispatch = Dispatch.pipe(Schema.decode({
  decode: SchemaGetter.checkEffect((_dispatch) => {
    sleep("5 millis")
    return true
  }),
  encode: SchemaGetter.passthrough()
}))
const ReviewedStandard = Schema.toStandardSchemaV1(ReviewedDispatch)

function validateDispatch(input: unknown) {
  return try((standard, input) => Promise.resolve(standard["~standard"].validate(input)), (cause) => new ValidationBoundaryError({ cause }), ReviewedStandard, input)
}

error ValidationBoundaryError {
  cause: unknown
}
```

The standard interface attempts synchronous validation first. If it encounters asynchronous work, it returns a Promise instead of a direct result. `Promise.resolve` inside an explicit foreign-code boundary normalizes both cases without `await` in Effeck. Prefer `Schema.decodeUnknownEffect` for ordinary Effeck application code; the standard interface is most useful for interoperability.

Inside the `try` hatch, plain TypeScript calls are allowed. `Promise.resolve` produces a Promise for either validation result shape, so no special mixed-return call rule is needed. The boundary maps synchronous exceptions and Promise rejections to `ValidationBoundaryError`; reported validation issues remain data.

## Handling defects during validation

```ts
const BrokenLabel = Schema.String.pipe(Schema.decode({
  decode: SchemaGetter.transformEffect((_label) => die("label invariant failed")),
  encode: SchemaGetter.passthrough()
}))
const BrokenStandard = Schema.toStandardSchemaV1(BrokenLabel)

function inspectBrokenValidation() {
  return try((standard) => standard["~standard"].validate("crate"), BrokenStandard)
}
```

The adapter reports an unexpected defect as one issue without a path, rather than exposing it as a normal field-validation failure. This is the adapter's behavior; ordinary Effeck `catch` still does not catch defects. Keep internal diagnostics at an application boundary if a defect needs investigation instead of relying only on the validation message.

Pass `leafHook`, `checkHook`, and `parseOptions` to customize the adapter. Its structured failures use the same issue formatter described in [Error Formatters](095-error-formatters.md).

[Upstream page](https://effect.website/docs/v4/schema/standard-schema)
