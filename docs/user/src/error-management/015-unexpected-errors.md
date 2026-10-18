# Unexpected Errors

Defects report bugs and broken invariants outside the typed error contract. Keep domain recovery focused on expected errors, and inspect defects at a boundary that can report or isolate the failed operation.

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

`Effect.die` terminates the computation with a defect. Prefer an `Error` with a useful message, so diagnostics include its stack. A JavaScript exception that no contract lists is also a defect; an explicit `throw` in an Effect function, including `throw new Error(...)`, is a typed failure. In a plain-contract function it is a JavaScript exception; it becomes a defect only if it escapes into Effect code (D263).

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

`Option.getOrThrow` and `Schema.decodeUnknownSync` are trusted calls whose exceptions are defects, so ordinary catch does not see them. A try block that runs Effects with E = `never` gets warning EK2004 by default; defects pass through its catch. Use a typed form such as `Effect.fromOption`, as above, or `Schema.decodeUnknownEffect` when callers need recovery. A JavaScript catch inside `Effect.gen` can see synchronous exceptions; an Effect catch in `.vibe` follows the expected-failure contract. A plain-contract function keeps JavaScript catch and receives every exception as `unknown` (D260).

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

`orDie` removes `E` by moving typed failures into defect reasons. This changes what callers can recover from; use it where a failure violates a required invariant. Map the error before promotion when the defect needs a clearer message.

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

`Effect.exit` returns `Exit<A, E>` as a successful value. A failure exit retains the full cause, including defects, interruption reasons, and combinations of reasons. This is broader than `Effect.result`, which exposes domain failures only; capturing an exit does not make an externally cancelled fiber immune to cancellation.

An `Exit` stays data when returned by a call. Constructing one with `Exit.succeed(value)` does not run it; calling a held exit runs that outcome.

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

`catchDefect` selects a defect and runs recovery; a cause with no defect propagates. On a mixed cause, recovery can consume the accompanying failures or interruption too, so use `catchCause` when every reason must inform that decision. Reraise defects you do not recognize. Effect `try`/`catch` does not intercept a defect, even if the defect value is an instance of a domain error class.

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

`exit(program)` returns the whole outcome, as `Effect.exit` does, so a `Failure` exit's cause holds every reason: failures, defects, and interruption that ordinary catch clauses leave alone. Test it with `is` or `match` on `Exit.Success` and `Exit.Failure`. `throw cause` restores the original failure, every reason included, as `Effect.failCause` does, instead of turning the cause object into a typed error. Inspect every reason before recovering from a mixed cause, and preserve cancellation when the boundary has no reason to consume it.
