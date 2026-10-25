# Matching

Matching turns an outcome into one value or selects the next computation. Use typed matching for domain failures, and cause matching when a boundary also needs defects and interruptions.

[Upstream page](https://effect.website/docs/v4/error-management/matching)

## Folding success and failure into values

```vibe
// run: Console.log(countLabel(3)); Console.log(countLabel(-1))
error InvalidCount

function itemCount(raw: number) {
  if (raw < 0) throw new InvalidCount()
  return raw
}

function countLabel(raw: number) {
  return Effect.match(itemCount(raw), {
    onFailure: () => "Count unavailable",
    onSuccess: (count) => `${count} items`
  })
}
// string!!
```

```output
3 items
Count unavailable
```

`Effect.match` applies synchronous value callbacks, which can't perform Effect operations; one that only formats text is eligible for plain JavaScript compilation. A cause without an expected failure propagates. A cause with a defect propagates too, its expected failures as defects (D305). From an expected failure beside an interruption, Effect selects the failure and discards the interruption, as ordinary catch does (D306).

## Choosing effectful branches

```vibe
// run: Console.log(reportCount(3)); Console.log(reportCount(-1))
function reportCount(raw: number) {
  return Effect.matchEffect(() => itemCount(raw), {
    onFailure: () => {
      Console.log("Invalid count received")
      return "Count unavailable"
    },
    onSuccess: (count) => {
      Console.log("Count accepted")
      return `${count} items`
    }
  })
}
```

```output
Count accepted
3 items
Invalid count received
Count unavailable
```

`matchEffect` accepts VibeLang functions that can pause, fail, or use services. Only the selected branch runs; its errors and requirements join the result's contract. Use `try`/`catch` when the branches read better as statements.

## Matching complete failure causes

```vibe
// run: Console.log(outcomeLabel()); Console.log(reportOutcome())
function outcomeLabel() {
  return Effect.matchCause(die(new Error("index invariant")), {
    onFailure: (cause) => cause.hasDies() ? "Index defect" : "Index stopped",
    onSuccess: () => "Index ready"
  })
}

function reportOutcome() {
  return Effect.matchCauseEffect(() => die(new Error("index invariant")), {
    onFailure: (cause) => {
      Console.log("Index stopped", JSON.stringify(cause.reasons.map((reason) => reason._tag)))
      return "Unavailable"
    },
    onSuccess: () => "Ready"
  })
}
```

```output
Index defect
Index stopped ["Die"]
Unavailable
```

`matchCause` passes the full cause to a synchronous failure callback; `matchCauseEffect` allows Effect operations in both callbacks. Both can consume cancellation and mixed causes, so use them where the boundary's policy calls for that.

## Discarding failures or all causes

```vibe
// run: ignoreRejectedCount(); discardBoundaryOutcome()
function ignoreRejectedCount() {
  Effect.ignore(itemCount(-2))
}
// void!!

function discardBoundaryOutcome() {
  Effect.ignoreCause(() => die(new Error("isolated task failed")))
}
// Output:
```

`ignore` discards the success value and typed failures; causes without an expected failure propagate, and so do causes with a defect (D305), but an expected failure beside an interruption is consumed with it. `ignoreCause` consumes the full failure cause, even one with no expected failure; use it only where losing both diagnostics and cancellation is deliberate.
