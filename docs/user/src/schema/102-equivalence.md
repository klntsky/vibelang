# Schema to Equivalence

Schema-derived equivalence works in Effeck exactly as in Effect. It compares decoded values using the structure described by a schema, making it useful for domain comparisons and round-trip checks. It does not decode or validate the compared values. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Deriving equality from schemas

```ts
const Shipment = Schema.Struct({
  reference: Schema.String,
  stops: Schema.Array(Schema.String)
})

function sameShipment(left: unknown, right: unknown) {
  const a = Schema.decodeUnknownEffect(Shipment)(left)
  const b = Schema.decodeUnknownEffect(Shipment)(right)
  return Effect.try(() => Schema.toEquivalence(Shipment)(a, b))
}
```

`toEquivalence` derives a comparison for the schema's decoded type. Arrays and nested structs are compared recursively. For a nonempty struct, only the fields described by the schema participate; unmodeled fields do not become extra equality conditions. Decode untrusted inputs before comparing them.

## Handling broad schemas and custom declarations

```ts
function sameUnknown(left: unknown, right: unknown) {
  return Effect.try(() => Schema.toEquivalence(Schema.Unknown)(left, right))
}

class ShipmentRef {
  constructor(readonly id: string, readonly label: string) {}
}
const ShipmentRefSchema = Schema.instanceOf(ShipmentRef, {
  toEquivalence: () => (left, right) => left.id === right.id
})

function sameRef() {
  return Effect.try(() => Schema.toEquivalence(ShipmentRefSchema)(
    new ShipmentRef("ref-61", "morning"),
    new ShipmentRef("ref-61", "afternoon")
  )) // true
}
```

`Any`, `Unknown`, `ObjectKeyword`, and the empty struct use `Equal.equals`, including its structural handling of arrays and objects. Opaque declarations also default to `Equal.equals`; add a `toEquivalence` annotation when domain identity follows a particular field. Parameterized declarations receive the derived comparator for each type parameter.

## Overriding generated comparison behavior

```ts
const ShipmentIdentity = Shipment.pipe(Schema.overrideToEquivalence(
  () => (left, right) => left.reference === right.reference
))

function sameIdentity(left: typeof Shipment.Type, right: typeof Shipment.Type) {
  return Effect.try(() => Schema.toEquivalence(ShipmentIdentity)(left, right))
}
```

`overrideToEquivalence` replaces the comparison derived at that schema node. This comparison intentionally ignores stops. Keep custom comparisons reflexive, symmetric, and transitive; otherwise consumers cannot treat them as an equivalence relation. A comparator callback is synchronous and should not call services or perform effects.

[Upstream page](https://effect.website/docs/v4/schema/equivalence)
