# Branded Types

`Brand` uses Effect's branded types in Effeck: a brand distinguishes domain values without changing their runtime representation. Use brands when two strings or numbers mean different things, and construct them at a boundary where their origin or validation is clear.

## Distinguishing identical representations

```ts
import { Brand } from "effect"

type InvoiceId = string & Brand.Brand<"InvoiceId">
type CustomerId = string & Brand.Brand<"CustomerId">

function invoiceUrl(id: InvoiceId) {
  return `/invoices/${id}`
}

function customerUrl(id: CustomerId) {
  return `/customers/${id}`
}
```

Two plain aliases for `string` remain interchangeable. These branded types require different compile-time evidence, so a `CustomerId` cannot be passed to `invoiceUrl`. The brand adds no field to the runtime string and does not itself validate external input.

## Nominal brands

```ts
import { Brand } from "effect"

type InvoiceId = string & Brand.Brand<"InvoiceId">
const InvoiceId = Brand.nominal<InvoiceId>()

function knownInvoice(raw: string) {
  const id = InvoiceId(raw)
  return { id, url: `/invoices/${id}` }
}
```

`Brand.nominal` builds a constructor that returns its input unchanged with the brand attached in its type. It performs no validation, so use it only where the domain meaning is already established. Brand builders and their returned constructors are trusted Effect library functions; call them directly. Exceptions from those plain calls are defects.

## Refined brands and validation

```ts
import { Brand } from "effect"

type WholeCents = number & Brand.Brand<"WholeCents">
const WholeCents = Brand.make<WholeCents>((n) =>
  Number.isSafeInteger(n) || "amount must be a safe integer"
)

class InvalidAmount extends TaggedError("InvalidAmount")<{
  readonly cause: unknown
}> {}

function parseAmount(raw: number) {
  return Effect.try({
    try: () => WholeCents(raw),
    catch: (cause) => new InvalidAmount({ cause })
  })
}
// Effect<WholeCents, InvalidAmount, never>
```

`Brand.make` accepts a synchronous check that returns `true` for valid input or an explanation for invalid input. The returned constructor throws a `Brand.BrandError` when validation fails. A direct call treats that exception as a defect, which ordinary `catch` does not see. Here `Effect.try` maps it to `InvalidAmount` in the expected error channel.

```ts
function inspectAmount(raw: number) {
  const optional = WholeCents.option(raw)
  const result = WholeCents.result(raw)
  const valid = WholeCents.is(raw)
  return { optional, result, valid }
}
```

`option` returns `Some` or `None`; `result` returns a `Result` whose failure contains `BrandError`; `is` returns a boolean and has a refinement signature. These trusted calls report invalid data without throwing. Use `Effect.fromResult(WholeCents.result(raw))` when invalid input should fail with `BrandError` in E. Unexpected exceptions remain defects.

## Composing constraints

```ts
import { Brand } from "effect"

type WholeCents = number & Brand.Brand<"WholeCents">
const WholeCents = Brand.make<WholeCents>((n) =>
  Number.isSafeInteger(n) || "amount must be a safe integer"
)

type Positive = number & Brand.Brand<"Positive">
const Positive = Brand.make<Positive>((n) => n > 0 || "amount must be positive")

const Charge = Brand.all(WholeCents, Positive)
type Charge = Brand.Brand.FromConstructor<typeof Charge>

class InvalidCharge extends TaggedError("InvalidCharge")<{
  readonly cause: unknown
}> {}

function chargeAmount(raw: number) {
  return Effect.try({
    try: () => Charge(raw),
    catch: (cause) => new InvalidCharge({ cause })
  })
}
// chargeAmount(125) succeeds; -5 and 1.5 fail with InvalidCharge.
```

`Brand.all` combines checks for constructors with the same underlying type. A valid result carries every constituent brand, so `Charge` satisfies both the integer and positivity requirements. Extracting the type from the constructor keeps the declared type aligned with the combined constraints.

## Coming from Effect

Brands keep their usual types and runtime behavior. Direct constructor exceptions are defects; use the constructor's `result` with `Effect.fromResult`, or an explicit `Effect.try` mapping, when invalid input should be an expected failure.
