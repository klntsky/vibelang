# Getting Started

`Schema` uses the same descriptions and interpreters in VibeLang as in Effect. Start with a struct, extract its types, then choose a parser whose failure behavior fits the caller. The [trusted call rules](085-introduction.md#configuring-compiler-prerequisites) apply to the plain Schema builders below.

## Declaring schemas

```vibe
// run: Console.log(JSON.stringify(receive({ tracking: "box-76", pieces: "2" })))
import { Console } from "effect"
const Shipment = Schema.Struct({
  tracking: Schema.NonEmptyString,
  pieces: Schema.FiniteFromString
})

function receive(input: unknown) {
  return Schema.decodeUnknownEffect(Shipment)(input)
}
```

```output
{"tracking":"box-76","pieces":2}
```

Each field describes its own representation and checks. The struct combines them into one codec: `pieces` arrives as text and becomes a number. Declaring `Shipment` builds a description and performs no decoding.

## Extracting decoded, encoded, and service types

```vibe
type Shipment = typeof Shipment.Type
type ShipmentWire = typeof Shipment.Encoded
type ReadServices = typeof Shipment.DecodingServices
type WriteServices = typeof Shipment.EncodingServices

type SameShipment = Schema.Schema.Type<typeof Shipment>
type SameWire = Schema.Codec.Encoded<typeof Shipment>
type SameReadServices = Schema.Codec.DecodingServices<typeof Shipment>
type SameWriteServices = Schema.Codec.EncodingServices<typeof Shipment>
```

The decoded type has a numeric `pieces`; the encoded type has a string. Both service types are `never` for this codec. Keep the directional service types when writing generic helpers so a future effectful conversion does not lose its requirements.

## Understanding default immutability

```vibe
// run: Console.log(JSON.stringify(Schema.decodeUnknownEffect(Batch)({ shipments: [{ tracking: "box-76", pieces: "2" }] })))
schema Batch {
  shipments: ReadonlyArray<Shipment>
}
// Batch: { readonly shipments: ReadonlyArray<Shipment> }
```

```output
{"shipments":[{"tracking":"box-76","pieces":2}]}
```

`schema Batch { ... }` declares the struct `Batch` and its decoded type, the interface `Batch`, in one declaration. A field names its type, and a schema such as `Shipment` stands for its own decoded type. Struct fields, arrays, records, and tuples are readonly by default. This describes their TypeScript types; it does not freeze JavaScript objects. Use explicit mutable schema combinators when mutation is part of the model.

## Choosing synchronous, result, or effect decoding

```vibe
// run: Console.log(decodeWithEffects({ tracking: "box-76", pieces: "2" }).pieces); Console.log(decodeAsData({ tracking: "box-76", pieces: "2" })); Console.log(decodeSynchronously({ tracking: "box-76", pieces: "2" }).pieces); try { decodeSynchronously({}) } catch (error) { Console.log(error._tag) }; Console.log(decodeAsData({}))
function decodeWithEffects(input: unknown) {
  return Schema.decodeUnknownEffect(Shipment)(input)
}

function decodeAsData(input: unknown) {
  const result = Schema.decodeUnknownResult(Shipment)(input)
  if (result.isFailure()) return result.failure.message
  return `received ${result.success.tracking}`
}

function decodeSynchronously(input: unknown) {
  return try((schema, input) => Schema.decodeUnknownSync(schema)(input), (cause) => new InvalidShipment({ cause }), Shipment, input)
}

error InvalidShipment {
  cause: unknown
}
```

```output
2
received box-76
2
InvalidShipment
Missing key
  at ["tracking"]
```

Prefer `decodeUnknownEffect` inside VibeLang: it can pause, requires the codec's decoding services, and fails with `Schema.SchemaError`. `decodeUnknownResult` returns success or failure as data and requires a synchronous, service-free decoding path. Unexpected exceptions from synchronous callbacks are defects; ordinary `catch` does not handle them.

`decodeUnknownSync` throws on invalid input. A direct call makes that exception a defect; the explicit boundary above turns it into `InvalidShipment`, which `catch` can handle. A `catch` around a direct call alone cannot run and produces warning EK2004. The `Exit` and `Option` variants provide other synchronous result shapes; an `Exit` stays data until called. The `Promise` variant supports asynchronous decoding without services and is consumed through the `try` hatch. The synchronous variants cannot complete asynchronous conversions, even when no service is needed.

## Encoding values

```vibe
// run: Console.log(JSON.stringify(send({ tracking: "box-76", pieces: 2 }))); try { sendUntrusted({ tracking: "box-76", pieces: "2" }) } catch (error) { Console.log(error._tag) }
function send(shipment: Shipment) {
  return Schema.encodeEffect(Shipment)(shipment)
}
// ShipmentWire!Schema.SchemaError!

function sendUntrusted(input: unknown) {
  return Schema.encodeUnknownEffect(Shipment)(input)
}
```

```output
{"tracking":"box-76","pieces":"2"}
SchemaError
```

Encoding reverses the conversion, including the numeric `pieces` field. `encodeEffect` accepts the decoded type; `encodeUnknownEffect` accepts unknown input and validates it. Encoding also has `Sync`, `Result`, `Option`, `Exit`, and `Promise` variants, with the same execution limits as decoding.

## Handling schema failures

```vibe
// run: Console.log(receiveOrExplain({ tracking: "box-76", pieces: "2" })); Console.log(receiveOrExplain({}))
function receiveOrExplain(input: unknown) {
  try {
    const shipment = receive(input)
    return `accepted ${shipment.tracking}`
  } catch (error) {
    return error.message
  }
}
// string!!
```

```output
accepted box-76
Missing key
  at ["tracking"]
```

The parser's expected failure is a `SchemaError` with `_tag: "SchemaError"`. Its `issue` preserves the structured failure and its `message` provides readable text. `catch` handles that expected failure; defects and interruption still propagate.

## Configuring excess properties and error collection

```vibe
// run: try { receiveStrict({ tracking: "box-76", pieces: "2", extra: true }) } catch (error) { Console.log(error._tag) }; Console.log(JSON.stringify(Schema.decodeUnknownEffect(ShipmentWithExtras)({ tracking: "box-76", pieces: "2", extra: true })))
function receiveStrict(input: unknown) {
  return Schema.decodeUnknownEffect(Shipment)(input, {
    onExcessProperty: "error",
    errors: "all"
  })
}

const ShipmentWithExtras = Schema.StructWithRest(Shipment, [
  Schema.Record(Schema.String, Schema.Unknown)
])
```

```output
SchemaError
{"tracking":"box-76","pieces":2,"extra":true}
```

By default, parsing removes unmodeled keys and stops at the first issue. `onExcessProperty: "error"` rejects extra keys, while `errors: "all"` collects independent issues encountered during parsing. To retain extra values, describe an index signature with `StructWithRest`.

Options can be supplied when creating the parser or when calling it; call options take precedence. Schema annotations do not override these options for nested nodes.

## Deriving guards and assertions

```vibe
// run: Console.log(shipmentTracking({ tracking: "box-76", pieces: 2 })); Console.log(shipmentTracking({ tracking: "box-76", pieces: "2" })); Console.log(assertShipment({ tracking: "box-76", pieces: 2 }))
const isShipment = Schema.is(Shipment)

function shipmentTracking(input: unknown) {
  if (isShipment(input)) return input.tracking
  return undefined
}

function assertShipment(input: unknown) {
  Schema.asserts(Shipment, input)
  return input.tracking
}
```

```output
box-76
undefined
box-76
```

Guards and assertions check the decoded type, so a numeric `pieces` is valid and a string `pieces` is not. `Schema.is` returns a type predicate; `Schema.asserts(schema, input)` throws a generic validation error whose `cause` contains the issue. These trusted calls preserve TypeScript's narrowing in the current function without a boundary. An assertion exception is a defect, so ordinary `catch` does not handle it. Use `decodeUnknownEffect` when invalid data should produce a typed validation failure.

## Coming from Effect

`Schema.decodeUnknownEffect(Shipment)(input)` gives a shipment directly inside a function. Use ordinary `try`/`catch` around it instead of generator-based error handling.

[Upstream page](https://effect.website/docs/v4/schema/getting-started)
