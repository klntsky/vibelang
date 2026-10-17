# Exit

`Exit` works in Effeck exactly as in Effect. Use `Exit<A, E>` to inspect success or an entire failure cause, including defects and interruption. A call that returns an Exit returns data; calling that data runs its recorded outcome.

## Representing full outcomes

```ts
error ExportRejected {
  reason: string
}

function inspectExport() {
  const outcome = Effect.exit {
    throw new ExportRejected({ reason: "empty selection" })
  }
  if (outcome.isFailure()) return outcome.cause.reasons.length
  return 0
}
```

An exit is `Success` with `.value`, or `Failure` with `.cause`. `Effect.exit` runs a program and returns that outcome as its success value. The caller can inspect it without re-raising the recorded failure.

## Creating success and cause-based failure exits

```ts
function sampleOutcomes() {
  const successful = Exit.succeed(12)
  const failed = Exit.failCause(
    Cause.fail("write rejected").combine(Cause.die(new Error("release failed")))
  )
  return { successful, failed }
}

function constantSuccess() {
  return Exit.succeed(12)
}
// Exit.Exit<number>!!
```

`Exit.succeed` and `Exit.failCause` construct exit data, including at a module's top level. `Exit.succeed(12)` gives a success exit containing `12`, and `Exit.failCause(cause)` gives a failure exit containing the cause. Neither raises a failure. Use `Effect.exit` to run a program and capture its actual outcome instead.

## Matching exit variants

```ts
function describe<A, E>(outcome: Exit.Exit<A, E>) {
  return outcome.match({
    onSuccess: (value) => `Completed: ${String(value)}`,
    onFailure: (cause) => Cause.pretty(cause)
  })
}
```

`Exit.match` receives the success value or the complete cause. Its callbacks here compute strings synchronously. Use `isSuccess` or `isFailure` and ordinary branches when the response needs effectful work.

## Relating exits to results

```ts
function asResult<A, E>(outcome: Exit.Exit<A, E>) {
  if (outcome.isSuccess()) return Result.succeed(outcome.value)
  return Result.fail(outcome.cause)
}
// Result.Result<A, Cause.Cause<E>>!!
```

An exit carries roughly the same information as `Result<A, Cause<E>>`. A `Result<A, E>` alone cannot express the additional defect and interruption reasons. Keep the full cause when moving an execution outcome into another data structure.

## Relating exits to effects

```ts
function replay<A, E>(outcome: Exit.Exit<A, E>) {
  return outcome()
}
// A!E!
```

Calling a held exit runs its constant outcome. Success returns the stored value; failure propagates the stored cause, without repeating the original work. Taking an exit with `exit(program)` and later calling it therefore separates inspection from propagation. Inside a try statement, `finally (exit)` binds the statement's own exit.

## Coming from Effect

`const outcome = Exit.succeed(value)` still builds an exit object. Call `outcome()` to propagate it, or use `Effect.exit(() => work())` to capture a program's outcome. Although Exit extends Effect, its constructors return data under the call rule.
