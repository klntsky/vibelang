# Cause

`Cause` works in Effeck exactly as in Effect. It records every reason a computation failed, including typed errors, defects, and interruption, so use it when the error channel alone cannot explain an outcome.

## Representing failure reasons

```ts
const expected = Cause.fail("record missing")
const defect = Cause.die(new Error("invariant broken"))
const interrupted = Cause.interrupt(17)
const empty = Cause.empty
empty.reasons.length // 0
```

`Cause.fail` stores an error in E; `Cause.die` stores a defect outside E. `Cause.interrupt` records interruption with an optional numeric fiber identifier. An empty cause has no reasons, rather than a separate `Empty` reason variant.

## Combining reasons

```ts
const combined = Cause.combine(
  Cause.fail("export failed"),
  Cause.die(new Error("cleanup failed"))
)
combined.reasons.map((reason) => reason._tag) // ["Fail", "Die"]
```

Effect 4 stores reasons in a flat array. `combine` retains both sets of reasons; it does not encode a sequential or parallel tree. A mixed cause must be inspected as a whole to avoid hiding a defect behind an expected failure.

## Extracting a computation's cause

```ts
class MissingRecord extends TaggedError("MissingRecord")<{
  readonly id: string
}> {}

function diagnose() {
  const outcome = Effect.exit(() => {
    throw new MissingRecord({ id: "r-9" })
  })
  if (Exit.isFailure(outcome)) {
    return Cause.pretty(outcome.cause)
  }
  return "completed"
}
```

`Effect.exit` runs the program and returns its full outcome as data. A failure exit exposes `.cause`, including defects and interruption that ordinary `catch` does not handle. Use `Effect.failCause(cause)` when you deliberately want to propagate the entire cause again.

## Identifying reason variants

```ts
function classify<E>(cause: Cause.Cause<E>) {
  const summary = {
    failed: Cause.hasFails(cause),
    defective: Cause.hasDies(cause),
    interrupted: Cause.hasInterrupts(cause),
    interruptionOnly: Cause.hasInterruptsOnly(cause)
  }
  for (const reason of cause.reasons) {
    if (Cause.isFailReason(reason)) Effect.log("expected failure")
    else if (Cause.isDieReason(reason)) Effect.log("defect")
    else if (Cause.isInterruptReason(reason)) {
      Effect.log(`interruption: ${reason.fiberId}`)
    }
  }
  return summary
}
```

Cause predicates answer whether a kind of reason is present. Reason guards narrow individual entries to `Fail`, `Die`, or `Interrupt`, exposing `.error`, `.defect`, or `.fiberId`. Check `reasons.length === 0` for an empty cause.

## Formatting and inspecting errors

```ts
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

`Cause.pretty` produces readable error and trace information; it returns an empty string for `Cause.empty`. Select individual reasons when a reporting boundary needs separate counts or categories. Format only errors suitable for the destination: error fields and messages can contain sensitive application data.

## Coming from Effect

An Effeck `catch` handles a cause only when it contains no defect and no interruption. For full outcome inspection, use `Effect.exit(() => work())` and keep the complete cause available.
