# Exit

`Exit` works in VibeLang exactly as in Effect. Use `Exit<A, E>` to inspect a success or an entire failure cause, defects and interruption included. A call that returns an Exit returns data; calling that data runs its recorded outcome.

## Representing full outcomes

```vibe
// run: Console.log(inspectExport())
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

```output
1
```

An exit is `Success` with `.value` or `Failure` with `.cause`. `Effect.exit` runs a program and succeeds with its outcome, which the caller inspects without re-raising the failure.

## Creating success and cause-based failure exits

```vibe
// run: const outcomes = sampleOutcomes(); Console.log(outcomes.successful.isSuccess(), outcomes.failed.isFailure(), constantSuccess().isSuccess()); if (outcomes.failed.isFailure()) Console.log(JSON.stringify(outcomes.failed.cause.reasons.map((reason) => reason._tag)))
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
// Effect<Exit.Exit<number>>
```

```output
true true true
["Fail","Die"]
```

`Exit.succeed` and `Exit.failCause` build exit data, even at a module's top level: `Exit.succeed(12)` is a success exit holding `12`, and `Exit.failCause(cause)` a failure exit holding the cause. Neither raises a failure.

## Matching exit variants

```vibe
// run: Console.log(describe(Exit.succeed(12))); Console.log(describe(Exit.failCause(Cause.empty)) === "")
function describe<A, E>(outcome: Exit.Exit<A, E>) {
  return match (outcome) {
    when Exit.Success(let value): `Completed: ${String(value)}`;
    when Exit.Failure(let cause): Cause.pretty(cause);
  }
}
```

```output
Completed: 12
true
```

`Exit.Success(let value)` binds the success value and `Exit.Failure(let cause)` the complete cause. Leaving out either arm is EK4701. Arms are ordinary expressions and can run Effect functions; `outcome.match({ onSuccess, onFailure })` also works when the callbacks compute values synchronously. To test one variant, write `outcome is Exit.Failure`.

## Relating exits to results

```vibe
// run: Console.log(asResult(Exit.succeed(12)).getOrThrow()); const failed = asResult(Exit.failCause(Cause.fail("rejected").combine(Cause.die("broken")))); if (failed.isFailure()) Console.log(JSON.stringify(failed.failure.reasons.map((reason) => reason._tag)))
function asResult<A, E>(outcome: Exit.Exit<A, E>) {
  if (outcome.isSuccess()) return Result.succeed(outcome.value)
  return Result.fail(outcome.cause)
}
// Effect<Result.Result<A, Cause.Cause<E>>>
```

```output
12
["Fail","Die"]
```

An exit carries roughly the information of `Result<A, Cause<E>>`; a `Result<A, E>` cannot express the defect and interruption reasons. Keep the full cause when you move an outcome into another data structure.

## Relating exits to effects

```vibe
// run: Console.log(replay(Exit.succeed(12))); const failed = Effect.exit(() => replay(Exit.failCause(Cause.fail("rejected").combine(Cause.die("broken"))))); if (failed.isFailure()) Console.log(JSON.stringify(failed.cause.reasons.map((reason) => reason._tag)))
function replay<A, E>(outcome: Exit.Exit<A, E>) {
  return outcome()
}
// Effect<A, E>
```

```output
12
["Fail","Die"]
```

Calling a held exit runs its constant outcome: success returns the stored value and failure propagates the stored cause, without repeating the original work. So taking an exit with `exit(program)` and calling it later separates inspection from propagation. In a try statement, `finally (exit)` binds the statement's own exit.

## Coming from Effect

`const outcome = Exit.succeed(value)` still builds an exit object; `outcome()` propagates it, and `Effect.exit(() => work())` captures a program's outcome. Exit extends Effect, but under the call rule its constructors return data.
