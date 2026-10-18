# Sandboxing

Sandboxing places the complete runtime failure cause in the typed error channel. Use it when an inspection pipeline needs to treat defects, interruption, and expected failures as explicit error data.

[Upstream page](https://effect.website/docs/v4/error-management/sandboxing)

## Moving causes into the error channel

```vibe
// run: Console.log(Effect.flip(() => readRow()).line); Console.log(JSON.stringify(Effect.flip(() => sandboxedRow()).reasons.map((reason) => reason._tag)))
error InvalidRow {
  line: number
}

function readRow() {
  throw new InvalidRow({ line: 8 })
}

function sandboxedRow() {
  return Effect.sandbox(() => readRow())
}
// void!Cause.Cause<InvalidRow>!
```

```output
8
["Fail"]
```

`Cause` is in scope in `.vibe` files. `sandbox` changes `A!E!R` into `A!Cause.Cause<E>!R`. A defect in the original computation becomes a `Die` reason inside a typed cause value rather than a defect reason at this outer level.

## Inspecting individual reasons

```vibe
// run: Console.log(JSON.stringify(rowFailureKinds()))
function rowFailureKinds() {
  try {
    return sandboxedRow()
  } catch (cause) {
    return cause.reasons.map((reason) => {
      if (reason.isFailReason()) return `row ${reason.error.line}`
      if (reason.isDieReason()) return "defect"
      return "interruption"
    })
  }
}
```

```output
["row 8"]
```

Effect 4 stores reasons in a flat readonly array. `isFailReason` exposes `error`, `isDieReason` exposes `defect`, and `isInterruptReason` identifies cancellation. Inspect the entire array because one outcome can contain more than one reason.

## Recovering selected variants

```vibe
// run: Console.log(rowOrPlaceholder())
function rowOrPlaceholder() {
  try {
    return sandboxedRow()
  } catch (cause) {
    if (!cause.hasDies() && !cause.hasInterrupts()) {
      return "Malformed row omitted"
    }
    return Effect.failCause(cause)
  }
}
```

```output
Malformed row omitted
```

This boundary recovers only causes made entirely of domain failures. Checking for any `Fail` reason alone would also consume defects or interruption in a mixed cause. Sandboxing makes that decision explicit; it does not remove the need for a recovery policy.

## Restoring causes after inspection

```vibe
// run: Console.log(Effect.flip(() => readWithInspection()).line)
function readWithInspection() {
  try {
    return sandboxedRow()
  } catch (cause) {
    Console.log("Row computation stopped", JSON.stringify(cause.reasons.map((reason) => reason._tag)))
    return Effect.failCause(cause)
  }
}
// void!InvalidRow!
```

```output
Row computation stopped ["Fail"]
8
```

`Effect.failCause(cause)` and `throw cause` both restore the original failure model, including every expected failure, defect, and interruption in the cause. To keep a cause as typed error data, use `Effect.fail(cause)` instead. For a single inspection or recovery step, `Effect.catchCause(() => readRow(), handler)` avoids changing the error channel first.
