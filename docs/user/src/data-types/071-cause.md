# Cause

`Cause` works in VibeLang exactly as in Effect. It records every reason a computation failed (typed errors, defects, interruption); use it when the error channel alone cannot explain an outcome.

## Representing failure reasons

```vibe
// run: Console.log(expected.reasons[0]._tag, defect.reasons[0]._tag, interrupted.reasons[0]._tag, empty.reasons.length)
const expected = Cause.fail("record missing")
const defect = Cause.die(new Error("invariant broken"))
const interrupted = Cause.interrupt(17)
const empty = Cause.empty
empty.reasons.length // 0
```

```output
Fail Die Interrupt 0
```

`Cause.fail` stores an error in E, `Cause.die` a defect outside E, and `Cause.interrupt` an interruption with an optional numeric fiber ID. An empty cause has no reasons; there is no `Empty` reason variant.

## Combining reasons

```vibe
// run: Console.log(JSON.stringify(combined.reasons.map((reason) => reason._tag))); Console.log(Cause.fail("same error").combine(Cause.fail("same error")).reasons.length)
const combined = Cause.fail("export failed").combine(Cause.die(new Error("cleanup failed")))
combined.reasons.map((reason) => reason._tag) // ["Fail", "Die"]
Cause.fail("same error").combine(Cause.fail("same error")).reasons.length // 1
```

```output
["Fail","Die"]
1
```

Effect 4 stores reasons in a flat array: `combine` merges both sets and drops equal duplicates, building no sequential or parallel tree. `combine` keeps an expected failure beside a defect, but where the runtime combines them, at a cleanup or a concurrent sibling, the failure becomes a defect of the same error (D305). Inspect a mixed cause as a whole.

## Extracting a computation's cause

```vibe
// run: Console.log(diagnose().split("\n")[0].trim())
error MissingRecord {
  id: string
}

function diagnose() {
  const outcome = exit {
    throw new MissingRecord({ id: "r-9" })
  }
  if (outcome.isFailure()) {
    return Cause.pretty(outcome.cause)
  }
  return "completed"
}
```

```output
MissingRecord:
```

`exit`, like `Effect.exit`, runs the program and returns its full outcome as data. A failure exit's `.cause` includes defects and interruption, which ordinary `catch` does not handle. `throw cause` propagates the entire cause again; to branch on the whole outcome, `match` the exit with `Exit.Success(let value)` and `Exit.Failure(let cause)` arms.

## Identifying reason variants

```vibe
// run: Console.log(JSON.stringify(classify(Cause.fail("missing").combine(Cause.die("broken")).combine(Cause.interrupt(17))))); Console.log(JSON.stringify(classify(Cause.empty)))
function classify<E>(cause: Cause.Cause<E>) {
  const summary = {
    failed: cause.hasFails(),
    defective: cause.hasDies(),
    interrupted: cause.hasInterrupts(),
    interruptionOnly: cause.hasInterruptsOnly()
  }
  for (const reason of cause.reasons) {
    if (reason.isFailReason()) Console.log("expected failure")
    else if (reason.isDieReason()) Console.log("defect")
    else if (reason.isInterruptReason()) {
      Console.log(`interruption: ${reason.fiberId}`)
    }
  }
  return summary
}
```

```output
expected failure
defect
interruption: 17
{"failed":true,"defective":true,"interrupted":true,"interruptionOnly":false}
{"failed":false,"defective":false,"interrupted":false,"interruptionOnly":false}
```

Cause predicates test whether a kind of reason is present. Reason guards narrow one entry to `Fail`, `Die`, or `Interrupt`, exposing `.error`, `.defect`, or `.fiberId`. An empty cause has `reasons.length === 0`.

## Formatting and inspecting errors

```vibe
// run: const result = inspect(Cause.fail("missing").combine(Cause.die("broken"))); Console.log(JSON.stringify({ failures: result.failures, defects: result.defects })); Console.log(JSON.stringify(inspect(Cause.empty)))
function inspect<E>(cause: Cause.Cause<E>) {
  const failures = cause.reasons
    .filter(Cause.isFailReason)
    .map((reason) => reason.error)
  const defects = cause.reasons
    .filter(Cause.isDieReason)
    .map((reason) => reason.defect)
  return { failures, defects, rendered: Cause.pretty(cause) }
}
```

```output
{"failures":["missing"],"defects":["broken"]}
{"failures":[],"defects":[],"rendered":""}
```

`Cause.pretty` renders errors and traces readably, and returns an empty string for `Cause.empty`. Select individual reasons when a reporting boundary needs separate counts or categories. Error fields and messages can hold sensitive application data, so format only errors fit for the destination.

## Coming from Effect

A VibeLang `catch` handles a cause only when it has no defect and no interruption. To inspect the full outcome, use `Effect.exit(() => work())`, which keeps the complete cause.
