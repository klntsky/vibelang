# Unexpected Errors

Defects report bugs and broken invariants outside the typed error contract. Keep domain recovery focused on expected errors, and inspect defects at a boundary that can report or isolate the failed operation.

[Upstream page](https://effect.website/docs/v4/error-management/unexpected-errors)

## Creating defects

```ts
function decodeOpcode(opcode: number) {
  if (opcode !== 1) die(new Error("unknown internal opcode"))
  return "start"
}
// string!!
```

`Effect.die` terminates the computation with a defect. Prefer an `Error` with a useful message, so diagnostics include its stack. A JavaScript exception that no contract lists is also a defect; an explicit Effeck `throw`, including `throw new Error(...)`, is a typed failure instead.

```ts
function firstLabel(labels: ReadonlyArray<string>) {
  try {
    return Effect.fromOption(Option.fromNullishOr(labels[0]))
  } catch (error) {
    return "No label"
  }
}
```

`Option.getOrThrow` and `Schema.decodeUnknownSync` are trusted calls whose exceptions are defects, so ordinary catch does not see them. A catch whose try block has no expected failures gets warning EK2004 by default. Use a typed form such as `Effect.fromOption`, as above, or `Schema.decodeUnknownEffect` when callers need recovery. A JavaScript catch inside `Effect.gen` can see synchronous exceptions; a catch in `.effeck` follows the expected-failure contract instead.

## Promoting typed failures to defects

```ts
error MissingSetting {
  name: string
}

function requireSetting(name: string) {
  throw new MissingSetting({ name })
}

function startup() {
  return Effect.orDie(requireSetting("region"))
}
// never!!

function startupWithMessage() {
  return requireSetting("region")
    |> Effect.mapError(%, (error) => new Error(`Startup needs ${error.name}`))
    |> Effect.orDie(%)
}
```

`orDie` removes `E` by moving typed failures into defect reasons. This changes what callers can recover from; use it where a failure violates a required invariant. Map the error before promotion when the defect needs a clearer message.

## Examining complete execution results

```ts
function inspectStartup() {
  const outcome = Effect.exit(startup())
  if (outcome.isFailure()) {
    return outcome.cause.hasDies() ? "startup defect" : "startup stopped"
  }
  return "started"
}
```

`Effect.exit` returns `Exit<A, E>` as a successful value. A failure exit retains the full cause, including defects, interruption reasons, and combinations of reasons. This is broader than `Effect.result`, which exposes domain failures only; capturing an exit does not make an externally cancelled fiber immune to cancellation.

An `Exit` stays data when returned by a call. Constructing one with `Exit.succeed(value)` does not run it; calling a held exit runs that outcome.

## Recovering defects at a boundary

```ts
function isolateDecoder(opcode: number) {
  return Effect.catchDefect(() => decodeOpcode(opcode), (defect) => {
    if (defect instanceof Error && defect.message === "unknown internal opcode") {
      log("Decoder disabled after an invariant failure")
      return "disabled"
    }
    return die(defect)
  })
}
```

`catchDefect` selects a defect and runs recovery; a cause with no defect propagates. On a mixed cause, recovery can consume the accompanying failures or interruption too, so use `catchCause` when every reason must inform that decision. Reraise defects you do not recognize. Ordinary `try`/`catch` does not intercept a defect, even if the defect value is an instance of a domain error class.

## Recovering a complete cause

```ts
function decoderBoundary(opcode: number) {
  try {
    return decodeOpcode(opcode)
  } catch (cause: Cause) {
    if (cause.hasInterrupts()) throw cause
    if (cause.hasDies()) {
      log("Decoder failed", cause.reasons.map((reason) => reason._tag))
      return "disabled"
    }
    throw cause
  }
}
```

A catch clause typed `Cause` receives every failure reason, as `Effect.catchCause` does, so it can deliberately handle outcomes that ordinary catch clauses leave alone. Clauses before it still see expected failures only. `throw cause` restores the original failure, every reason included, as `Effect.failCause` does, instead of turning the cause object into a typed error. Inspect every reason before recovering from a mixed cause, and preserve cancellation when the boundary has no reason to consume it.
