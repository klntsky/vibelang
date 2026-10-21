# Creating Effects

An ordinary function describes a VibeLang computation: return a value to succeed, throw a typed value for an expected failure, and use explicit Effect boundaries for plain TypeScript libraries.

## Success and failure

```vibe
// run: Console.log(reserve("notebook", true).item)
error Unavailable {
  item: string
}

function reserve(item: string, available: boolean) {
  if (!available) throw new Unavailable({ item })
  return { item, reserved: true }
}
// { item: string; reserved: boolean }!Unavailable!
// Output: notebook
```

The compiler records the returned object in `A` and `Unavailable` in `E`. `Effect.succeed(value)` and `Effect.fail(error)` still work inside a function, but run immediately. To store the work, write `const reservation = () => reserve("notebook", true)`.

## Track errors as values

```vibe
// run: Console.log(attemptReservation("notebook", false))
function attemptReservation(item: string, available: boolean) {
  const outcome = Effect.result(reserve(item, available))
  if (outcome.isFailure()) return outcome.failure.item
  return outcome.success.item
}
// string!!
// Output: notebook
```

`Effect.result` captures an expected failure as a `Result` value so the function can continue; defects and interruption still propagate. In an Effect function, `throw` adds to the inferred contract, and `Effect.fail` gives the same expected failure channel.

## Wrap synchronous operations

```vibe
// Assume ./legacy-codec.ts exports encode(value: string): string.
import { encode } from "./legacy-codec.ts"

error EncodingFailed {
  cause: unknown
}

function encodeLabel(value: string) {
  return try((value) => encode(value), (cause) => new EncodingFailed({ cause }), value)
}
// string!EncodingFailed!
```

In an Effect function, a plain imported function needs a boundary or a trusted contract that says what its exceptions mean. `try` is a boundary hatch: `try((value) => encode(value), value)` turns an exception into the expected failure `UnknownError`, and a function in second place, as above, gives the failure a domain name.

The hatch's first argument is plain TypeScript, evaluated when the Effect runs: calls in it are not Effects, and platform APIs are allowed. It is self-contained: it may use its own parameters and locals, globals, imports such as `encode`, and the file's error declarations, but no other binding of the `.vibe` file, such as the parameter `value`. Pass such a value as an input, an argument after the function and the error mapper; it fills the function's first parameter. EK4405 reports a binding read from outside; its quick fix passes the binding as an input.

JavaScript language built-ins have compiler trust contracts, so you call operations such as string trimming directly. Exports of `effect` and `@effect/*` packages are trusted too: data operations such as `Option.map` and `Schema.is` are plain calls whose exceptions are defects.

A synchronous TypeScript function can declare trust once with `/** @vibelang trusted */` on its declaration. Each `@throws {T}` names an exported exception class whose instances become expected failures; unlisted exceptions stay defects. Re-export a function from a package you cannot edit through a tagged `.ts` module. The tag is not allowed on Promise- or Effect-returning functions. Calls that rely on tagged expected failures report migration warning EK4406 in the current compiler (D249). For new adapters, export a `trusted(implementation, [ErrorClass])` value from `vibelang-runtime` in the `.ts` module and call it from `.vibe`; it returns an Effect with the listed expected failures. A `.vibe` file can call `trusted` too, on an imported TypeScript function ([Expected errors](../error-management/014-expected-errors.md#trusting-a-typescript-function)).

In Effect code, `catch` sees expected failures only, not defects from `Option.getOrThrow` or `Schema.decodeUnknownSync`; for recoverable input, use `Option.getOrElse`, `Effect.fromOption`, or `Schema.decodeUnknownEffect`. A catch whose try block has E = `never` gets EK2004 (defects pass through), a warning by default. A plain function's `catch` follows the same rule (D282): it receives typed failures only, so an exception from TypeScript code passes through it. Plain foreign calls at module top level are ordinary JavaScript calls, so their exceptions fail the module's import.

`sync` wraps synchronous work whose exceptions should be defects: `sync(crypto.randomUUID())`, or `sync(() => { … })` for several statements. A small adapter can live in a `.vibe` hatch or a [module declaration](#write-typescript-in-a-module-declaration); keep larger foreign-library adapters in `.ts`. `.vibe` helpers that only calculate or mutate local data need no wrapper. Use `Console` or `Effect.log` for output and `FileSystem` for files.

## Create effects from promises

```vibe
// Assume this SDK function returns Promise<string> and accepts AbortSignal.
import { lookupLabel } from "./legacy-sdk.ts"

error LookupFailed {
  cause: unknown
}

function label(id: string) {
  return try((id, signal) => lookupLabel(id, signal), (cause) => new LookupFailed({ cause }), id)
}
// string!LookupFailed!
```

`try` with a function that returns a promise, `async` or not, waits for it and maps a rejection into `E`; without a second argument the failure is `UnknownError`. The function receives an `AbortSignal` after its inputs. Pass it to an API that supports cancellation; otherwise interruption stops the wait but not necessarily the foreign operation.

`sync` with such a function treats rejection as a defect; it belongs in an adapter whose success-only promise contract is deliberate. `async` appears only inside a hatch: a VibeLang function waits by calling, `join` waits for a fiber, and `await` appears only in the await operations. For HTTP, use Effect's HTTP modules, not a wrapped browser `fetch` call.

## Adapt callbacks and cancellation

```vibe
// A one-shot read ends its registration when either callback fires.
// run: Console.log(readAndReport(sync(() => ({ read: (onValue: (n: number) => void) => { onValue(7); return () => {} } }))))
type Source = {
  read: (onValue: (n: number) => void, onError: (e: Error) => void) => () => void
}

function nextReading(source: Source) {
  return callback((source, resume: Resume<number, Error>) => {
    return source.read((n) => resume(n), (error) => resume.fail(error))
  }, source)
}

function readAndReport(source: Source) {
  const reading = nextReading(source)
  Console.log(`reading=${reading}`)
  return reading
}
// Output: reading=7
// Output: 7
```

`callback` is the boundary hatch for a callback API. It registers a plain callback and waits for its first completion: `resume(value)` succeeds, `resume.fail(error)` fails with an expected failure, and `resume.exit(exit)` adopts an `Exit`; later completions are ignored. A function the registration returns runs if the wait is interrupted, so this adapter cancels the one-shot read on interruption. The registration receives its inputs (here `source`), then `resume`, then an `AbortSignal`. Annotate `resume` with the success and failure types, `Resume<number, Error>`, because calls to `resume` in the body cannot infer them. The body is plain, self-contained TypeScript: it may call platform APIs but reads `source` only as an input.

For resource acquisition, cancellation must also release a resource that arrives after the waiter is gone; ignoring a late callback is not enough. Prefer an existing Effect resource API, or a TypeScript adapter using the runtime's `suspend`, which reports whether delivery lost the cancellation race.

## Write TypeScript in a module declaration

```vibe
// run: Console.log(slugOf("  Hello, World  ")); Console.log(Effect.result(slugOf("  ?! "))._tag)
module slugs {
  // TypeScript: throw, try and catch, and regular expressions keep their JavaScript meaning.
  export function slugify(title: string): string {
    const slug = title.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "")
    if (slug === "") throw new Error(`no slug in ${JSON.stringify(title)}`)
    return slug
  }
}

import { slugify } from slugs

error NoSlug {
  title: string
}

function slugOf(title: string) {
  return try((title) => slugify(title), () => new NoSlug({ title }), title)
}
// Output: hello-world
// Output: Failure
```

`module slugs { … }` declares a module whose body is TypeScript, checked and compiled as a `.ts` file would be (D283). There `throw`, `try` and `catch`, `async` and `await`, and platform APIs keep their TypeScript meaning, and VibeLang's own syntax, such as `A!E` or an `error` declaration, is a syntax error. The file reaches the module's exports only through an import whose specifier is the module's name, `import { slugify } from slugs`, so a hatch uses them as it uses any import, and a call outside a hatch is a plain foreign call, as for an import from a `.ts` file. `export { slugify } from slugs` re-exports one, and `import slugs` runs a module for its effects. The body imports what it needs itself, its relative specifiers resolve from the `.vibe` file's directory, and it cannot read the `.vibe` file's bindings, so `title` reaches `slugify` as a hatch input.

Declare modules at a file's top level. In a `.vibe` file `module` never declares a namespace: write `namespace` for one. The emitted JavaScript holds each module the file imports inline, run once before the file's own code, and each module's declarations are a declaration file of their own, `main.vibe.slugs.d.ts` beside `main.d.ts` (D303). A module the file imports only types from emits nothing. Move an adapter to its own `.ts` file when other files need it.

## Suspend work

```vibe
// run: Console.log(JSON.stringify(counters()))
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
// Output: [1,2]
```

The returned function delays its body until each call. For retry to create new state each time, wrap the whole computation, mutable setup included. `Effect.suspend` still delays building an Effect in TypeScript adapters; in VibeLang, passing a program already delays its body.

```vibe
// run: Console.log(countdown(3))
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
// Output: 3
```

Suspending recursion uses runtime frames, not JavaScript stack frames. Pure recursion without Effect operations uses the JavaScript stack and can overflow. Returns and throws also unify success and failure channels across branches without an extra suspension wrapper.

## Coming from Effect

Use returns, throws, and program functions for local work. Keep the hatches `sync`, `try`, and `callback`, and TypeScript callback adapters, where they state a foreign-code boundary.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/creating-effects)
