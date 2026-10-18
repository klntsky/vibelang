# Branded Types

`Brand` uses Effect's branded types in VibeLang: a brand distinguishes domain values without changing their runtime representation. Use brands when two strings or numbers mean different things, and construct them at a boundary where their origin or validation is clear.

## Distinguishing identical representations

```vibe
// run: Console.log(invoiceUrl("inv-3" as InvoiceId), customerUrl("customer-5" as CustomerId))
type InvoiceId = string & Brand.Brand<"InvoiceId">
type CustomerId = string & Brand.Brand<"CustomerId">

function invoiceUrl(id: InvoiceId) {
  return `/invoices/${id}`
}

function customerUrl(id: CustomerId) {
  return `/customers/${id}`
}
```

```output
/invoices/inv-3 /customers/customer-5
```

Two plain aliases for `string` remain interchangeable. These branded types require different compile-time evidence, so a `CustomerId` cannot be passed to `invoiceUrl`. The brand adds no field to the runtime string and does not itself validate external input.

## Nominal brands

```vibe
// run: Console.log(JSON.stringify(knownInvoice("inv-3")))
type InvoiceId = string & Brand.Brand<"InvoiceId">
const InvoiceId = Brand.nominal<InvoiceId>()

function knownInvoice(raw: string) {
  const id = InvoiceId(raw)
  return { id, url: `/invoices/${id}` }
}
```

```output
{"id":"inv-3","url":"/invoices/inv-3"}
```

`Brand.nominal` builds a constructor that returns its input unchanged with the brand attached in its type. It performs no validation, so use it only where the domain meaning is already established. Brand builders and their returned constructors are trusted Effect library functions; call them directly. Exceptions from those plain calls are defects.

## Refined brands and validation

```vibe
// run: Console.log(parseAmount(125)); try { parseAmount(1.5) } catch (error) { Console.log(error._tag) }
type WholeCents = number & Brand.Brand<"WholeCents">
const WholeCents = Brand.make<WholeCents>((n) =>
  Number.isSafeInteger(n) || "amount must be a safe integer"
)

error InvalidAmount {
  cause: unknown
}

function parseAmount(raw: number) {
  return try((make, raw) => make(raw), (cause) => new InvalidAmount({ cause }), WholeCents, raw)
}
// WholeCents!InvalidAmount!
```

```output
125
InvalidAmount
```

`Brand.make` accepts a synchronous check that returns `true` for valid input or an explanation for invalid input. The returned constructor throws a `Brand.BrandError` when validation fails. A direct call treats that exception as a defect, which ordinary `catch` does not see. Here the `try` hatch maps it to `InvalidAmount` in the expected error channel.

```vibe
// run: const valid = inspectAmount(125); const invalid = inspectAmount(1.5); Console.log(valid.optional.isSome(), valid.result.isSuccess(), valid.valid); Console.log(invalid.optional.isNone(), invalid.result.isFailure(), invalid.valid)
function inspectAmount(raw: number) {
  const optional = WholeCents.option(raw)
  const result = WholeCents.result(raw)
  const valid = WholeCents.is(raw)
  return { optional, result, valid }
}
```

```output
true true true
true true false
```

`option` returns `Some` or `None`; `result` returns a `Result` whose failure contains `BrandError`; `is` returns a boolean and has a refinement signature. These trusted calls report invalid data without throwing. Use `Effect.fromResult(WholeCents.result(raw))` when invalid input should fail with `BrandError` in E. Unexpected exceptions remain defects.

## Composing constraints

```vibe
// run: Console.log(chargeAmount(125)); for (const input of [-5, 1.5]) { try { chargeAmount(input) } catch (error) { Console.log(error._tag) } }
type WholeCents = number & Brand.Brand<"WholeCents">
const WholeCents = Brand.make<WholeCents>((n) =>
  Number.isSafeInteger(n) || "amount must be a safe integer"
)

type Positive = number & Brand.Brand<"Positive">
const Positive = Brand.make<Positive>((n) => n > 0 || "amount must be positive")

const Charge = Brand.all(WholeCents, Positive)
type Charge = Brand.Brand.FromConstructor<typeof Charge>

error InvalidCharge {
  cause: unknown
}

function chargeAmount(raw: number) {
  return try((make, raw) => make(raw), (cause) => new InvalidCharge({ cause }), Charge, raw)
}
// chargeAmount(125) succeeds; -5 and 1.5 fail with InvalidCharge.
```

```output
125
InvalidCharge
InvalidCharge
```

`Brand.all` combines checks for constructors with the same underlying type. A valid result carries every constituent brand, so `Charge` satisfies both the integer and positivity requirements. Extracting the type from the constructor keeps the declared type aligned with the combined constraints.

## Coming from Effect

Brands keep their usual types and runtime behavior. Direct constructor exceptions are defects; use the constructor's `result` with `Effect.fromResult`, or an explicit `try` mapping, when invalid input should be an expected failure.
