# Two Types of Errors

A missing record and a broken invariant need different responses: VibeLang tracks recoverable domain failures in `Effect<A, E, R>`, and defects in the runtime's failure cause.

[Upstream page](https://effect.website/docs/v4/error-management/two-error-types)

## Separating failures from defects

```vibe
// run: Console.log(displayPrice("notebook")); Console.log(displayPrice("pen"))
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

```output
$12
No price for pen
```

`throw` declares an expected failure. `try`/`catch` recovers it, and the caught value's type is inferred from the protected code. A defect, such as an invalid property access or `Effect.die(new Error("broken invariant"))`, bypasses `catch`.

Expected failures also come from a callee's `E` and from exceptions a trusted contract lists; an unlisted exception is a defect. When a cause holds several expected failures and nothing else, `catch` receives the first and handles the whole cause; `Effect.catchCause` inspects every reason.

Interruption, a third kind of termination, also bypasses `catch`. Defects and interruption still run `finally` and release `using` resources. A cause that mixes a failure with a defect or interruption passes through ordinary `catch` whole.

## Choosing the error type

```vibe
// run: Console.log(percentage(1, 4)); Console.log(Effect.matchCause(() => percentage(1, 0), { onFailure: (cause) => cause.hasDies(), onSuccess: () => false }))
function percentage(part: number, total: number) {
  if (total <= 0) {
    die(new Error("validated total must be positive"))
  }
  return part / total * 100
}
// number!!
```

```output
25
true
```

Use this contract only when validation already established the invariant; if users can submit an invalid total, throw a typed error. Put a failure in `E` when callers have a useful response, such as asking for corrected input or choosing another source.

`E = never` promises that no expected failure escapes, not that bugs or cancellation can't occur. A boundary that must examine those uses `Effect.exit`, `Effect.catchDefect`, or `Effect.catchCause`.

Precisely, `E` describes failures in causes made entirely of expected failures. A defect turns the expected failures beside it into defects (D305), but a cause that mixes them with an interruption passes through a catch and can keep failure values outside `E`, which no ordinary catch recovers.

## Coming from Effect

- Write `throw error` where a generator would yield a failure.
- Use ordinary `try`/`catch` for domain recovery and `finally` for cleanup.
