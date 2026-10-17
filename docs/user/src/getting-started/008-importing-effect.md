# Importing Effect

Effect's modules are already in scope in `.effeck` files: the prelude, and every module that `effect` exports from its root, such as `Console`, `Option`, `Stream`, and `Schedule`. Import modules from `effect`'s subpath packages, such as `effect/http`, as you need them.

## Install the core package

```sh
npm install effect@4.0.5
```

The core package supplies Effect's data types, schedules, concurrency tools, and other modules. See [Installation](004-installation.md) for platform packages, `effeck`, and `effeck-runtime`.

## Module and namespace imports

```ts
function pauseBeforeRetry() {
  const started = Clock.currentTimeMillis()
  Effect.sleep(Duration.millis(25))
  return started
}

const backoff = Schedule.exponential("50 millis")
```

`Effect`, the `Effect<A, E, R>` type, `Schema`, `Layer`, `Scope`, `TaggedError`, and `Control` need no import. In a contract, `Scope` means Effect's `Scope.Scope`, as in `File!IOError!Scope`. The prelude also includes `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, and `abandon`; the boundary hatches `sync`, `try`, and `callback`; and the runners `runPromise`, `runFork`, `runSync`, and `runCallback`. `Clock`, `Duration`, `Schedule`, and every other module of `effect`'s root need no import either, and neither do `flow` and `identity`.

JavaScript's own globals keep their names. `Array`, `String`, `Number`, `Boolean`, `Symbol`, `BigInt`, `Function`, and `RegExp` mean JavaScript's, so import Effect's modules with those names under another name:

```ts
import { Array as Arr } from "effect"

function firstTwo(values: ReadonlyArray<number>) {
  return Arr.take(values, 2)
}
```

A host's globals do not shadow Effect's modules: with the DOM in your `lib`, `Option` is still Effect's. An import or a local declaration with the same name as a module shadows it, as `import * as Duration from "effect/Duration"` would. Value constructors such as `Duration.millis` build data; `Clock.currentTimeMillis()` and `Effect.sleep(...)` run computations. Effect's own packages are trusted, so their data functions need no foreign-call boundary; exceptions from those functions are defects.

## Compose standalone functions

```ts
function expirationWindow() {
  const short = Duration.seconds(15)
  return Duration.toMillis(short)
}
// number!!
```

Effect exposes operations as module functions. Call them with their data argument or use their curried form in `pipe` where available. An Effect pipeline starts with a program, as in `pipe(() => work(), Effect.timeout("1 second"))`.

## Bundling unused code

Named module imports and direct namespace imports both expose the library without adding methods to your objects. Bundlers can eliminate unused exports when their module analysis supports it. Check the generated bundle when size matters; the import spelling alone does not guarantee a particular size.

Effeck's global names are source conveniences. They refer to Effect and language support code, which still participates in the emitted module graph.

## Reusable operators

```ts
function withinSecond<A, E, R>(program: () => A!E!R) {
  return Effect.timeout(program, "1 second")
}

function example() {
  return withinSecond(() => {
    Effect.sleep("10 millis")
    return "ready"
  })
}
```

A helper can accept a program and apply library operators to it. Its contract preserves the program's errors and services and adds `TimeoutError` here. You can extend your application without changing Effect's prototypes.

## Coming from Effect

Skip imports from `effect`'s root in `.effeck`. In `.ts` integration files, import `Effect`, `Layer`, and the other names you use as usual.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/importing-effect)
