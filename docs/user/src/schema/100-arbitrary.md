# Schema to Arbitrary

Effect's native `Arbitrary` module derives generators and shrinkers from schemas in Effeck exactly as in Effect. Use it to explore valid inputs and check properties across many examples. The module and replay format are unstable in Effect 4.0.5. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Generating schema-conforming random values

```ts
const Parcel = Schema.Struct({
  label: Schema.NonEmptyString,
  count: Schema.Int.check(Schema.isBetween({ minimum: 1, maximum: 24 }))
})

function sampleParcels() {
  const generator = try(Arbitrary.schema(Parcel))
  return Arbitrary.sampleEffect(generator, { count: 12, seed: "parcel-samples" })
}
```

`Arbitrary.schema` derives a generator immediately and can throw when derivation is unsupported, so the example puts it inside a boundary. `sampleEffect` runs generation and accepts `count`, `size`, `maxDiscards`, and `seed`. A seed makes sampling reproducible; `size` is a local complexity budget for each collection or recursive component, not one global maximum length.

## Translating filters into generation constraints

```ts
const EvenCapacity = Schema.Number.check(Schema.makeFilter(
  (value) => Number.isInteger(value) && value >= 2 && value <= 40 && value % 2 === 0,
  {
    expected: "an even capacity from 2 to 40",
    arbitraryConstraint: {
      order: Order.Number,
      minimum: 2,
      maximum: 40,
      number: "integer"
    }
  }
))
```

Built-in bounds, lengths, patterns, property counts, and uniqueness checks provide constructive constraints where supported. Custom `arbitraryConstraint` metadata guides construction but does not replace the predicate. The example constructs integers in the desired range and lets the predicate reject odd candidates. Every generated root and shrink candidate still passes the schema's filters.

## Handling opaque checks and transformed values

```ts
function sampleRepresentations() {
  const decoded = try(Arbitrary.schema(Schema.FiniteFromString))
  const encoded = try(Arbitrary.schema(Schema.toEncoded(Schema.FiniteFromString)))
  const numbers = Arbitrary.sampleEffect(decoded, { count: 4, seed: 17 })
  const strings = Arbitrary.sampleEffect(encoded, { count: 4, seed: 17 })
  return { numbers, strings }
}
```

Schema generation targets the decoded type, so a string-to-number codec generates numbers. Derive from `toEncoded` to generate the input representation instead. Opaque predicates without constructive metadata still run, but rejected candidates consume the discard budget. Sampling then fails with `SampleError` when the budget is exhausted; property checking instead returns `Exhausted`.

## Checking properties

```ts
function checkCountRoundTrip() {
  const Count = Schema.Int.check(Schema.isBetween({ minimum: 1, maximum: 100 }))
  const generator = try(Arbitrary.schema(Count))
  const result = Arbitrary.checkEffect(generator, (count) => {
    const wire = Schema.encodeEffect(Schema.FiniteFromString)(count)
    return Schema.decodeEffect(Schema.FiniteFromString)(wire) === count
  }, { runs: 200, seed: "count-round-trip" })
  return result
}
```

`checkEffect` accepts a pure boolean property or an effectful boolean property. False results and expected failures are shrinkable falsifications; defects and interruption fail the checking Effect itself. Its result is `Passed`, `Falsified`, `Exhausted`, or `ReplayMismatch`.

A falsification includes `shrunkInput` and a `replay` token. Call `checkEffect` with the same property and `{ replay: result.replay }` to reproduce it; the token controls the seed, attempt, size, and shrink path, overriding other checking options. `formatCheckFailure` returns a diagnostic for unsuccessful results and undefined for `Passed`.

## Composing generators and customizing shrinking

```ts
function sampleDispatchPlans() {
  const counts = try(() => Arbitrary.schema(Schema.Int.check(
    Schema.isBetween({ minimum: 0, maximum: 32 })
  ), {
    shrink: (value) => value === 0 ? [] : [Math.floor(value / 2)]
  }))
  const plans = Arbitrary.all({
    count: counts,
    label: Arbitrary.Constant("test-dispatch")
  })
  return Arbitrary.sampleEffect(plans, { count: 6, seed: 91 })
}
```

`map` transforms values and shrink candidates; `flatMap` selects dependent generators; `all` combines a tuple, iterable, or record. `filter` and `filterMap` discard rejected values, while `Constant` produces one value with no shrink candidates. Prefer constructive schema constraints when valid values are rare.

A custom `shrink` replaces the derived shrink strategy without replacing root generation. It must be synchronous, deterministic, terminating, and free of mutation. Candidates are validated against the schema before the property sees them. Treat generated values as immutable because checking, shrinking, and replay can examine them repeatedly.

## Supporting custom declarations

```ts
class ShelfRef {
  constructor(readonly code: `shelf_${string}`) {}
}
const ShelfRefSchema = Schema.instanceOf(ShelfRef, {
  toCodecArbitrary: () => Schema.link<ShelfRef>()(
    Schema.TemplateLiteral(["shelf_", Schema.String]),
    SchemaTransformation.transform({
      decode: (code) => new ShelfRef(code),
      encode: (ref) => ref.code
    })
  )
})

function sampleShelves() {
  const generator = try(Arbitrary.schema(ShelfRefSchema))
  return Arbitrary.sampleEffect(generator, { count: 5, seed: 23 })
}
```

An opaque declaration needs a generatable representation through `toCodecArbitrary`. The link converts generated representation values into domain values. Parameterized declarations receive decoded parameter schemas and normalized constraints in their annotation callback.

## Diagnosing derivation failures

```ts
function tryImpossibleGenerator() {
  try {
    return try(Arbitrary.schema(Schema.Never))
  } catch (error) {
    log("Arbitrary derivation failed for the empty domain")
    throw error
  }
}
```

Derivation fails immediately for impossible constraints, `Never`, unsupported declarations, and recursion with no finite generation path. Runtime exhaustion is different: inspect the effective seed and discard budget, then add constructive constraints or correct the predicate. Preserve the seed and replay token with a failing test so the same counterexample can be examined again.

[Upstream page](https://effect.website/docs/v4/schema/arbitrary)
