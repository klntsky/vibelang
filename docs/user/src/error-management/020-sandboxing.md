# Sandboxing

Sandboxing places the complete runtime failure cause in the typed error channel. Use it when an inspection pipeline needs to treat defects, interruption, and expected failures as explicit error data.

[Upstream page](https://effect.website/docs/v4/error-management/sandboxing)

## Moving causes into the error channel

```ts
error InvalidRow {
  line: number
}

function readRow() {
  throw new InvalidRow({ line: 8 })
}

function sandboxedRow() {
  return Effect.sandbox(() => readRow())
}
// never!Cause.Cause<InvalidRow>!
```

Import `Cause` from `effect` when naming the contract explicitly. `sandbox` changes `A!E!R` into `A!Cause.Cause<E>!R`. A defect in the original computation becomes a `Die` reason inside a typed cause value rather than a defect reason at this outer level.

## Inspecting individual reasons

```ts
function rowFailureKinds() {
  try {
    return sandboxedRow()
  } catch (cause) {
    return cause.reasons.map((reason) => {
      if (Cause.isFailReason(reason)) return `row ${reason.error.line}`
      if (Cause.isDieReason(reason)) return "defect"
      return "interruption"
    })
  }
}
```

Effect 4 stores reasons in a flat readonly array. `isFailReason` exposes `error`, `isDieReason` exposes `defect`, and `isInterruptReason` identifies cancellation. Inspect the entire array because one outcome can contain more than one reason.

## Recovering selected variants

```ts
function rowOrPlaceholder() {
  try {
    return sandboxedRow()
  } catch (cause) {
    if (!Cause.hasDies(cause) && !Cause.hasInterrupts(cause)) {
      return "Malformed row omitted"
    }
    return Effect.failCause(cause)
  }
}
```

This boundary recovers only causes made entirely of domain failures. Checking for any `Fail` reason alone would also consume defects or interruption in a mixed cause. Sandboxing makes that decision explicit; it does not remove the need for a recovery policy.

## Restoring causes after inspection

```ts
function readWithInspection() {
  try {
    return sandboxedRow()
  } catch (cause) {
    log("Row computation stopped", cause.reasons.map((reason) => reason._tag))
    return Effect.failCause(cause)
  }
}
// never!InvalidRow!
```

`Effect.failCause(cause)` restores the original failure model, including any defects and interruptions. `throw cause` would instead keep the cause object as a typed error. For a single inspection or recovery step, `Effect.catchCause(() => readRow(), handler)` avoids changing the error channel first.
