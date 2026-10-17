# Error Channel Operations

Error-channel operations transform, observe, or expose failures without immediately replacing the whole computation. Ordinary statements remain useful for local validation and selective recovery.

[Upstream page](https://effect.website/docs/v4/error-management/error-channel-operations)

## Mapping failures or both outcomes

```ts
error InvalidQuantity {
  value: number
}

function quantity(value: number) {
  if (value < 1) throw new InvalidQuantity({ value })
  return value
}

function publicQuantity(value: number) {
  return Effect.mapError(
    () => quantity(value),
    (error) => `Quantity must be positive: ${error.value}`
  )
}
// number!string!

function quantityMessage(value: number) {
  return Effect.mapBoth(() => quantity(value), {
    onFailure: (error) => `Rejected quantity ${error.value}`,
    onSuccess: (count) => `${count} units`
  })
}
// string!string!
```

`mapError` changes `E` and preserves successes. `mapBoth` maps each outcome channel separately; it does not turn failures into successes. The callbacks must be eligible synchronous functions; use a catch block or an effectful operator when a transformation needs Effect operations.

These combinators keep Effect's own failure-selection policy: when a mixed cause contains an expected failure, mapping it can discard the other reasons. Ordinary catch maps only causes made entirely of expected failures and preserves mixed causes intact.

`mapErrorEager` and `mapBothEager` are eager optimization variants. They preserve the same channel distinction and are not substitutes for effectful callbacks.

## Rejecting successes with a predicate

```ts
function boundedQuantity(value: number) {
  return Effect.filterOrFail(
    () => quantity(value),
    (count) => count <= 20,
    (count) => `Maximum is 20; received ${count}`
  )
}

function cappedQuantity(value: number) {
  return Effect.filterOrElse(
    () => quantity(value),
    (count) => count <= 20,
    () => 20
  )
}
```

`filterOrFail` keeps a success that passes the predicate and otherwise adds the generated error to `E`. A type guard can narrow the success type. `filterOrElse` runs a replacement computation on a rejected success, so that branch can use services or fail.

A normal `if` followed by `throw` is the statement form. Neither form changes existing failures from the source.

## Observing errors, tags, causes, and defects

```ts
function observeQuantity(value: number) {
  return quantity(value)
    |> Effect.tapError(%, (error) => log(`Rejected quantity ${error.value}`))
    |> Effect.tapErrorTag(%, "InvalidQuantity", () => log("Quantity validation failed"))
    |> Effect.tapCause(%, (cause) => {
      log("Failure kinds", cause.reasons.map((reason) => reason._tag))
    })
}

function observeDefect() {
  return Effect.tapDefect(
    () => die(new Error("inventory invariant")),
    () => log("Inventory defect recorded")
  )
}
```

`tapError` observes typed errors, and `tapErrorTag` selects a tagged member without handling it. `tapCause` sees the full cause, including interruption and combined reasons; `tapDefect` observes defect values. On a successful observation, the original outcome propagates.

If observation fails, its failure can change the resulting cause, so use observation code that cannot fail when preserving the original outcome matters. Log diagnostic fields rather than entire customer values or credentials. A catch block that logs and rethrows is useful for local typed error observation.

## Exposing failures as successes

```ts
function inspectQuantity(value: number) {
  const result = Effect.result(quantity(value))
  const summary = result.match({
    onFailure: () => "Rejected",
    onSuccess: (count) => `Accepted ${count}`
  })
  const exit = Effect.exit(quantity(value))
  return { summary, completed: exit.isSuccess() }
}

function quantityOrErrorValue(value: number) {
  return Effect.catch(() => quantity(value), (error) => error)
}
// number | InvalidQuantity!!
```

`result` exposes typed errors as `Result` data; `exit` captures the complete outcome with its cause. The last example instead puts either the original number or the original error directly into one success union. Removing `E` this way does not mean an error has been resolved at the application level.

## Swapping channels

```ts
function rejectedQuantity() {
  return Effect.flip(() => quantity(0))
}
// InvalidQuantity!number!
```

`flip` makes typed errors successful values and successful values typed failures. A cause without an expected failure still propagates; a mixed cause containing one can become a success and lose its other reasons. This is useful for inspecting an error in a focused test, while `mapError` and catch usually express production transformations more clearly.
