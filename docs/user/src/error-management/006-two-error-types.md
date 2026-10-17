# Two Types of Errors

A missing record and a broken invariant need different responses. Effeck tracks recoverable domain failures in `Effect<A, E, R>` and keeps defects in the runtime's failure cause.

[Upstream page](https://effect.website/docs/v4/error-management/two-error-types)

## Separating failures from defects

```ts
error UnknownProduct {
  sku: string
}

function price(sku: string) {
  if (sku !== "notebook") throw new UnknownProduct({ sku })
  return 12
}
// number!UnknownProduct!

function displayPrice(sku: string) {
  try {
    return `$${price(sku)}`
  } catch (error) {
    return `No price for ${error.sku}`
  }
}
```

`throw` declares an expected failure. A caller can recover with `try`/`catch`, and the caught value has the error type inferred from the protected code. A defect, such as an invalid property access or `Effect.die(new Error("broken invariant"))`, bypasses this catch.

Expected failures also come from a callee's `E` and exceptions listed by a trusted contract. An exception that no contract lists is a defect. When a cause contains several expected failures and nothing else, `catch` receives the first and handles the cause as a whole; use `Effect.catchCause` to inspect every reason.

Interruption is a third kind of termination. It also bypasses `catch`; both interruption and defects still run `finally` and release `using` resources. If a cause contains a failure together with a defect or interruption, ordinary `catch` propagates the entire cause.

## Choosing the error type

```ts
function percentage(part: number, total: number) {
  if (total <= 0) {
    Effect.die(new Error("validated total must be positive"))
  }
  return part / total * 100
}
// number!!
```

Use this contract only when validation has already established the invariant. If users can submit an invalid total, represent that case with a typed error instead. Put a failure in `E` when callers have a useful response, such as asking for corrected input or choosing another source.

`E = never` promises no expected failures escape. It does not promise the absence of bugs or cancellation. Use `Effect.exit`, `Effect.catchDefect`, or `Effect.catchCause` at a boundary that must examine those outcomes.

Precisely, `E` describes failures in causes made entirely of expected failures. A mixed cause can retain failure values outside `E` after passing through a catch; no ordinary catch recovers those values.

## Coming from Effect

- Write `throw error` where a generator would yield a failure.
- Use ordinary `try`/`catch` for domain recovery and `finally` for cleanup.
