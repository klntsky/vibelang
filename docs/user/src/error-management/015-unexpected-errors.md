# Unexpected Errors

Defects report bugs and broken invariants, outside the typed error contract. Domain recovery handles expected errors; inspect defects at a boundary that can report or isolate the failed operation.

[Upstream page](https://effect.website/docs/v4/error-management/unexpected-errors)

## Creating defects

```vibe
// run: Console.log(decodeOpcode(1)); Console.log(Effect.matchCause(() => decodeOpcode(2), { onFailure: (cause) => cause.hasDies(), onSuccess: () => false }))
function decodeOpcode(opcode: number) {
  if (opcode !== 1) die(new Error("unknown internal opcode"))
  return "start"
}
// string!!
```

```output
start
true
```

`Effect.die` terminates the computation with a defect, preferably an `Error` with a useful message so diagnostics include its stack. A JavaScript exception no contract lists is also a defect. An explicit `throw` in an Effect function, even `throw new Error(...)`, is a typed failure; it is a typed failure in a plain-annotated function too, where it may not escape: a plain function never fails (D282). Only a hatch's TypeScript keeps JavaScript's `throw`.

```vibe
// run: Console.log(firstLabel(["one"])); Console.log(firstLabel([]))
function firstLabel(labels: ReadonlyArray<string>) {
  try {
    return Effect.fromOption(Option.fromNullishOr(labels[0]))
  } catch (error) {
    return "No label"
  }
}
```

```output
one
No label
```

`Option.getOrThrow` and `Schema.decodeUnknownSync` are trusted calls whose exceptions are defects, which ordinary catch doesn't see. A catch whose try block has E = `never` gets warning EK2004 by default; defects pass through it. When callers need recovery, use a typed form: `Effect.fromOption`, as above, or `Schema.decodeUnknownEffect`. A JavaScript catch inside `Effect.gen` sees synchronous exceptions; an Effect catch in `.vibe` follows the expected-failure contract. A plain function's catch follows the same contract (D282).

## Promoting typed failures to defects

```vibe
// run: Console.log(Effect.matchCause(() => startup(), { onFailure: (cause) => cause.hasDies(), onSuccess: () => false })); Console.log(Effect.matchCause(() => startupWithMessage(), { onFailure: (cause) => cause.hasDies(), onSuccess: () => false }))
error MissingSetting {
  name: string
}

function requireSetting(name: string) {
  throw new MissingSetting({ name })
}

function startup() {
  return Effect.orDie(requireSetting("region"))
}
// void!!

function startupWithMessage() {
  return requireSetting("region")
    |> Effect.mapError(%, (error) => new Error(`Startup needs ${error.name}`))
    |> Effect.orDie(%)
}
```

```output
true
true
```

`orDie` removes `E` by moving typed failures into defect reasons, which callers can't recover from; use it where a failure violates a required invariant. For a clearer defect message, map the error first.

## Examining complete execution results

```vibe
// run: Console.log(inspectStartup())
function inspectStartup() {
  const outcome = Effect.exit(startup())
  if (outcome.isFailure()) {
    return outcome.cause.hasDies() ? "startup defect" : "startup stopped"
  }
  return "started"
}
```

```output
startup defect
```

`Effect.exit` succeeds with an `Exit<A, E>`. A failure exit keeps the full cause: defects, interruption reasons, and combinations of reasons. `Effect.result` exposes domain failures only. Capturing an exit doesn't make an externally cancelled fiber immune to cancellation.

An `Exit` returned by a call stays data, and `Exit.succeed(value)` constructs one without running it; calling a held exit runs that outcome.

## Recovering defects at a boundary

```vibe
// run: Console.log(isolateDecoder(1)); Console.log(isolateDecoder(2))
function isolateDecoder(opcode: number) {
  return Effect.catchDefect(() => decodeOpcode(opcode), (defect) => {
    if (defect instanceof Error && defect.message === "unknown internal opcode") {
      Console.log("Decoder disabled after an invariant failure")
      return "disabled"
    }
    return die(defect)
  })
}
```

```output
start
Decoder disabled after an invariant failure
disabled
```

`catchDefect` selects a defect and runs recovery; a cause with no defect propagates. On a mixed cause, recovery can also consume the accompanying failures or interruption, so use `catchCause` when every reason must inform the decision. Reraise defects you don't recognize. Effect `try`/`catch` never intercepts a defect, even one that is an instance of a domain error class.

## Recovering a complete cause

```vibe
// run: Console.log(decoderBoundary(1)); Console.log(decoderBoundary(2))
function decoderBoundary(opcode: number) {
  const outcome = exit(decodeOpcode(opcode))
  if (outcome is Exit.Success(let value)) return value
  const cause = outcome.cause
  if (cause.hasInterrupts()) throw cause
  if (cause.hasDies()) {
    Console.log("Decoder failed", JSON.stringify(cause.reasons.map((reason) => reason._tag)))
    return "disabled"
  }
  throw cause
}
```

```output
start
Decoder failed ["Die"]
disabled
```

`exit(program)` returns the whole outcome, as `Effect.exit` does: a `Failure` exit's cause holds every reason, including the failures, defects, and interruption that ordinary catch leaves alone. Test it with `is` or `match` on `Exit.Success` and `Exit.Failure`. `throw cause` restores the original failure with every reason, as `Effect.failCause` does; it doesn't make the cause object a typed error. Inspect every reason before recovering from a mixed cause, and preserve cancellation unless the boundary has a reason to consume it.
