# Error Channel Operations

Error-channel operations transform, observe, or expose failures without immediately replacing the whole computation. Ordinary statements still suit local validation and selective recovery.

[Upstream page](https://effect.website/docs/v4/error-management/error-channel-operations)

## Mapping failures or both outcomes

```vibe
// run: Console.log(publicQuantity(2)); Console.log(Effect.flip(() => publicQuantity(0))); Console.log(quantityMessage(2)); Console.log(Effect.flip(() => quantityMessage(0)))
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

```output
2
Quantity must be positive: 0
2 units
Rejected quantity 0
```

`mapError` changes `E` and preserves successes. `mapBoth` maps each outcome channel separately; failures stay failures. The callbacks must be eligible synchronous functions; for a transformation that needs Effect operations, use a catch block or an effectful operator.

These combinators keep Effect's own policy: `mapError` maps every expected failure in place and keeps defects and interruption. A cause with a defect has no expected failure left to map (D305). Ordinary catch recovers the first expected failure of a cause with no defect (D306) and leaves the others intact.

`mapErrorEager` and `mapBothEager` are eager optimization variants with the same channel distinction, not substitutes for effectful callbacks.

## Rejecting successes with a predicate

```vibe
// run: Console.log(boundedQuantity(20)); Console.log(Effect.flip(() => boundedQuantity(21))); Console.log(cappedQuantity(21)); Console.log(Effect.flip(() => cappedQuantity(0))._tag)
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

```output
20
Maximum is 20; received 21
20
InvalidQuantity
```

`filterOrFail` keeps a success that passes the predicate and otherwise adds the generated error to `E`; a type guard can narrow the success type. `filterOrElse` runs a replacement computation on a rejected success, so that branch can use services or fail.

The statement form is an `if` followed by `throw`. Neither form changes the source's existing failures.

## Observing errors, tags, causes, and defects

```vibe
// run: Console.log(observeQuantity(2)); Console.log(Effect.flip(() => observeQuantity(0))._tag); Console.log(Effect.matchCause(() => observeDefect(), { onFailure: (cause) => cause.hasDies(), onSuccess: () => false }))
function observeQuantity(value: number) {
  return quantity(value)
    |> Effect.tapError(%, (error) => Console.log(`Rejected quantity ${error.value}`))
    |> Effect.tapErrorTag(%, "InvalidQuantity", () => Console.log("Quantity validation failed"))
    |> Effect.tapCause(%, (cause) => {
      Console.log("Failure kinds", JSON.stringify(cause.reasons.map((reason) => reason._tag)))
    })
}

function observeDefect() {
  return Effect.tapDefect(
    () => die(new Error("inventory invariant")),
    () => Console.log("Inventory defect recorded")
  )
}
```

```output
2
Rejected quantity 0
Quantity validation failed
Failure kinds ["Fail"]
InvalidQuantity
Inventory defect recorded
true
```

`tapError` observes typed errors; `tapErrorTag` selects a tagged member without handling it; `tapCause` sees the full cause, interruption and combined reasons included; `tapDefect` observes defect values. After a successful observation, the original outcome propagates.

A failing observation can change the resulting cause, so observe with code that can't fail when preserving the original outcome matters. Log diagnostic fields, not entire customer values or credentials. For local typed error observation, a catch block can log and rethrow.

## Exposing failures as successes

```vibe
// run: Console.log(JSON.stringify(inspectQuantity(2))); Console.log(JSON.stringify(inspectQuantity(0))); Console.log(quantityOrErrorValue(2)); const value = quantityOrErrorValue(0); Console.log(typeof value === "number" ? value : value._tag)
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

```output
{"summary":"Accepted 2","completed":true}
{"summary":"Rejected","completed":false}
2
InvalidQuantity
```

`result` exposes typed errors as `Result` data; `exit` captures the complete outcome with its cause. The last example puts the original number or error directly into one success union; removing `E` this way doesn't resolve the error at the application level.

## Swapping channels

```vibe
// run: Console.log(rejectedQuantity()._tag)
function rejectedQuantity() {
  return Effect.flip(() => quantity(0))
}
// InvalidQuantity!number!
```

```output
InvalidQuantity
```

`flip` swaps typed errors and successful values. A cause without an expected failure still propagates, and so does one with a defect, its failures as defects (D305); an expected failure beside an interruption becomes a success and loses the interruption. Use it to inspect an error in a focused test; `mapError` and catch usually express production transformations more clearly.
