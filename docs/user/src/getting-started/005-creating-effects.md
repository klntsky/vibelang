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
// { item: string; reserved: boolean }!Unavailable!
```

The compiler records the returned object in `A` and `Unavailable` in `E`. `Effect.succeed(value)` and `Effect.fail(error)` remain available inside a function, but those calls run immediately. To store the work, write `const reservation = () => reserve("notebook", true)`.

## Track errors as values

```ts
function attemptReservation(item: string, available: boolean) {
  const outcome = Effect.result(reserve(item, available))
  if (outcome.isFailure()) return outcome.failure.item
  return outcome.success.item
}
// string!!
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
  return try(encode(value), (cause) => new EncodingFailed({ cause }))
}
// string!EncodingFailed!
```

A plain imported function needs a boundary or a trusted contract that declares what its exceptions mean. `try` is a boundary hatch: `try(encode(value))` turns an exception into the expected failure `UnknownError`, and a second argument, as above, gives the failure a domain name. The hatch's argument is plain TypeScript, evaluated when the Effect runs: calls there do not run as Effects, and platform APIs are allowed. JavaScript language built-ins have compiler trust contracts, so operations such as string trimming can be called directly. Exports of `effect` and `@effect/*` packages are trusted too: data operations such as `Option.map` and `Schema.is` are plain calls, and their exceptions are defects.

A synchronous TypeScript function can declare trust once with `/** @effeck trusted */` on its declaration. Each `@throws {T}` names an exported exception class whose instances become expected failures; unlisted exceptions remain defects. Re-export a function from a package you cannot edit through a tagged `.ts` module. The tag is not allowed on Promise- or Effect-returning functions.

`catch` sees expected failures only. It does not catch defects from `Option.getOrThrow` or `Schema.decodeUnknownSync`; use `Option.getOrElse`, `Effect.fromOption`, or `Schema.decodeUnknownEffect` for recoverable input. A `catch` whose `try` can raise no expected failure is EK2004, a warning by default. Plain foreign calls at module top level are ordinary JavaScript calls, so their exceptions fail the module's import.

`sync` wraps synchronous work whose exceptions should be defects: `sync(crypto.randomUUID())`, or `sync(() => { … })` for several statements. A small adapter can live in `.effeck` inside a hatch; keep larger foreign-library adapters in `.ts`. Ordinary `.effeck` helpers that only calculate or mutate local data need no wrapper. Use `Console` or `Effect.log` for output and `FileSystem` for files.

## Create effects from promises

```ts
// Assume this SDK function returns Promise<string> and accepts AbortSignal.
import { lookupLabel } from "./legacy-sdk.ts"

error LookupFailed {
  cause: unknown
}

function label(id: string) {
  return try((signal) => lookupLabel(id, signal), (cause) => new LookupFailed({ cause }))
}
// string!LookupFailed!
```

`try` with a function that returns a promise, `async` or not, waits for it and maps a rejection into `E`. Pass its signal to an API that supports cancellation; otherwise interruption stops waiting without necessarily stopping the foreign operation. Without a second argument the failure is `UnknownError`.

`sync` with such a function treats rejection as a defect and belongs in an adapter whose success-only promise contract is deliberate. `async` appears only inside a hatch: an Effeck function waits by calling, `join` waits for a fiber, and `await` appears only in the await operations. Use Effect's HTTP modules for HTTP rather than wrapping a browser `fetch` call.

## Adapt callbacks and cancellation

```ts
// A one-shot read ends its registration when either callback fires.
type Source = {
  read: (onValue: (n: number) => void, onError: (e: Error) => void) => () => void
}

function nextReading(source: Source) {
  return callback<number, Error>((resume) =>
    source.read((n) => resume(n), (error) => resume.fail(error))
  )
}

function readAndReport(source: Source) {
  const reading = nextReading(source)
  log(`reading=${reading}`)
  return reading
}
```

`callback` is the boundary hatch for a callback API. It registers a plain callback and waits for its first completion: `resume(value)` succeeds, `resume.fail(error)` fails with an expected failure, and `resume.exit(exit)` adopts an `Exit`. Later completions are ignored. A function that the registration returns runs if waiting is interrupted, so this adapter cancels the one-shot read on interruption. The registration also receives an `AbortSignal` as its second argument. Name the success and failure types, `callback<number, Error>`, because calls to `resume` inside the body cannot infer them. The body is plain TypeScript, so it may call platform APIs.

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
// number[]!!; returns [1, 2]
```

The returned function delays its body until each call. Wrap an entire computation, including mutable setup, if retry should create new state each time. `Effect.suspend` remains useful in TypeScript adapters to delay construction of an Effect; in Effeck, passing a program already delays its body.

```ts
function countdown(n: number): number!! {
  if (n <= 0) return 0
  sleep("1 millis")
  return 1 + countdown(n - 1)
}

function checkedQuantity(n: number) {
  if (n < 0) throw new Unavailable({ item: "quantity" })
  return n
}
// number!Unavailable!
```

Suspending recursion uses runtime frames, so it does not accumulate JavaScript stack frames. Pure recursion without Effect operations uses the JavaScript stack and can overflow. Ordinary returns and throws also unify success and failure channels across branches without an extra suspension wrapper.

## Coming from Effect

Use returns, throws, and program functions for local work. Keep the hatches `sync`, `try`, and `callback`, and TypeScript callback adapters where they state a foreign-code boundary.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/creating-effects)
