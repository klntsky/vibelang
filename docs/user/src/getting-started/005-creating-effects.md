# Creating Effects

Write an ordinary function to describe an Effeck computation. Return a value for success, throw a typed value for an expected failure, and use explicit Effect boundaries when integrating plain TypeScript libraries.

## Success and failure

```ts
error Unavailable {
  item: string
}

function reserve(item: string, available: boolean) {
  if (!available) throw new Unavailable({ item })
  return { item, reserved: true }
}
// Effect<{ item: string; reserved: boolean }, Unavailable, never>
```

The compiler records the returned object in `A` and `Unavailable` in `E`. `Effect.succeed(value)` and `Effect.fail(error)` remain available inside a function, but those calls run immediately. To store the work, write `const reservation = () => reserve("notebook", true)`.

## Track errors as values

```ts
function attemptReservation(item: string, available: boolean) {
  const outcome = Effect.result(() => reserve(item, available))
  if (Result.isFailure(outcome)) return outcome.failure.item
  return outcome.success.item
}
// Effect<string, never, never>
```

`Effect.result` captures an expected failure as a `Result` value so the function can continue. Defects and interruption still propagate. Unlike an unchecked JavaScript exception, an Effeck `throw` contributes to the inferred contract; using `Effect.fail` gives the same expected failure channel.

## Wrap synchronous operations

```ts
// Assume ./legacy-codec.ts exports encode(value: string): string.
import { encode } from "./legacy-codec.ts"

error EncodingFailed {
  cause: unknown
}

function encodeLabel(value: string) {
  return Effect.try({
    try: () => encode(value),
    catch: (cause) => new EncodingFailed({ cause })
  })
}
// Effect<string, EncodingFailed, never>
```

A plain imported function needs a boundary or a trusted contract that declares what its exceptions mean. The short form `Effect.try(() => encode(value))` uses `UnknownError`; the object form gives the error a domain name. JavaScript language built-ins have compiler trust contracts, so operations such as string trimming can be called directly. Exports of `effect` and `@effect/*` packages are trusted too: data operations such as `Option.map` and `Schema.is` are plain calls, and their exceptions are defects.

A synchronous TypeScript function can declare trust once with `/** @effeck trusted */` on its declaration. Each `@throws {T}` names an exported exception class whose instances become expected failures; unlisted exceptions remain defects. Re-export a function from a package you cannot edit through a tagged `.ts` module. The tag is not allowed on Promise- or Effect-returning functions.

`catch` sees expected failures only. It does not catch defects from `Option.getOrThrow` or `Schema.decodeUnknownSync`; use `Option.getOrElse`, `Effect.fromOption`, or `Schema.decodeUnknownEffect` for recoverable input. A `catch` whose `try` can raise no expected failure is EK2004, a warning by default. Plain foreign calls at module top level are ordinary JavaScript calls, so their exceptions fail the module's import.

`Effect.sync` wraps synchronous work whose exceptions should be defects. Keep raw foreign-library adapters in `.ts` when they need unchecked callbacks or platform access; ordinary `.effeck` helpers that only calculate or mutate local data need no wrapper. Use `Console` or `Effect.log` for output and `FileSystem` for files.

## Create effects from promises

```ts
// Assume this SDK function returns Promise<string> and accepts AbortSignal.
import { lookupLabel } from "./legacy-sdk.ts"

error LookupFailed {
  cause: unknown
}

function label(id: string) {
  return Effect.tryPromise({
    try: (signal) => lookupLabel(id, signal),
    catch: (cause) => new LookupFailed({ cause })
  })
}
// Effect<string, LookupFailed, never>
```

`Effect.tryPromise` waits for the promise and maps rejections into `E`. Pass its signal to an API that supports cancellation; otherwise interruption stops waiting without necessarily stopping the foreign operation. The short overload uses `UnknownError`.

`Effect.promise` treats rejection as a defect and belongs in an adapter whose success-only promise contract is deliberate. There is no `async` or `await` in `.effeck` files. Use Effect's HTTP modules for HTTP rather than wrapping a browser `fetch` call.

## Adapt callbacks and cancellation

```ts
// TypeScript (.ts)
// next-reading.ts
import { Effect } from "effect"

// A one-shot read ends its registration when either callback fires.
type Source = {
  read: (onValue: (n: number) => void, onError: (e: Error) => void) => () => void
}

export function nextReading(source: Source) {
  return Effect.callback<number, Error>((complete) => {
    const cancel = source.read(
      (n) => complete(Effect.succeed(n)),
      (error) => complete(Effect.fail(error))
    )
    return Effect.sync(cancel)
  })
}
```

```ts
import { nextReading } from "./next-reading.ts"

function readAndReport(source: Parameters<typeof nextReading>[0]) {
  const reading = nextReading(source)
  Effect.log(`reading=${reading}`)
  return reading
}
```

`Effect.callback` registers a plain callback and waits for its first completion. Later completion calls are ignored; the returned Effect runs if waiting is interrupted, so this adapter cancels the one-shot read on interruption. Its registration function can also receive an `AbortSignal` as its second argument.

This adapter keeps registration in TypeScript. A plain-value adapter can also use `Effect.callback` in `.effeck`: its registration function is an explicit boundary that permits plain TypeScript calls. Complete it with `Exit.succeed(value)` or `Exit.fail(error)`, which build data without running it. Platform calls and uncontrolled clock or randomness reads remain rejected inside boundaries.

For resource acquisition, cancellation must also release a resource that arrives after the waiter has gone away; merely ignoring a late callback is insufficient. Prefer an existing Effect resource API, or a TypeScript adapter using the runtime's `suspend`, which reports whether delivery lost the cancellation race.

## Suspend work

```ts
function nextCounter() {
  let count = 0
  return () => {
    count += 1
    return count
  }
}

function counters() {
  const next = nextCounter()
  return [next(), next()]
}
// Effect<number[], never, never>; returns [1, 2]
```

The returned function delays its body until each call. Wrap an entire computation, including mutable setup, if retry should create new state each time. `Effect.suspend` remains useful in TypeScript adapters to delay construction of an Effect; in Effeck, passing a program already delays its body.

```ts
function countdown(n: number): Effect<number, never, never> {
  if (n <= 0) return 0
  Effect.sleep("1 millis")
  return 1 + countdown(n - 1)
}

function checkedQuantity(n: number) {
  if (n < 0) throw new Unavailable({ item: "quantity" })
  return n
}
// Effect<number, Unavailable, never>
```

Suspending recursion uses runtime frames, so it does not accumulate JavaScript stack frames. Pure recursion without Effect operations uses the JavaScript stack and can overflow. Ordinary returns and throws also unify success and failure channels across branches without an extra suspension wrapper.

## Coming from Effect

Use returns, throws, and program functions for local work. Keep constructors such as `Effect.try`, `Effect.tryPromise`, and TypeScript callback adapters where they state a foreign-code boundary.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/creating-effects)
