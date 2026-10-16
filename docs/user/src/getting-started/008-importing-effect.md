# Importing Effect

EffectScript uses Effect's modules and standard module import syntax. A small prelude is already in scope in `.es` files; import the other modules as you need them.

## Install the core package

```sh
npm install effect@4.0.5
```

The core package supplies Effect's data types, schedules, concurrency tools, and other modules. See [Installation](004-installation.md) for platform packages, `@effectscript/compiler`, and `@effectscript/runtime`.

## Module and namespace imports

```ts
import { Clock, Duration, Schedule } from "effect"

function pauseBeforeRetry() {
  const started = Clock.currentTimeMillis()
  Effect.sleep(Duration.millis(25))
  return started
}

const backoff = Schedule.exponential("50 millis")
```

`Effect`, the `Effect<A, E, R>` type, `Schema`, `Layer`, `Scope`, `TaggedError`, and `Control` need no import. In a contract, `Scope` means Effect's `Scope.Scope`, as in `Effect<File, IOError, Scope>`. The prelude also includes `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, and `abandon`. `Clock`, `Duration`, and `Schedule` are ordinary imports.

A namespace import is also available: `import * as Duration from "effect/Duration"`. Choose one style consistently. Value constructors such as `Duration.millis` build data; `Clock.currentTimeMillis()` and `Effect.sleep(...)` run computations. Effect's own packages are trusted, so their data functions need no foreign-call boundary; exceptions from those functions are defects.

## Compose standalone functions

```ts
import { Duration } from "effect"

function expirationWindow() {
  const short = Duration.seconds(15)
  return Duration.toMillis(short)
}
// Effect<number, never, never>
```

Effect exposes operations as module functions. Call them with their data argument or use their curried form in `pipe` where available. An Effect pipeline starts with a program, as in `pipe(() => work(), Effect.timeout("1 second"))`.

## Bundling unused code

Named module imports and direct namespace imports both expose the library without adding methods to your objects. Bundlers can eliminate unused exports when their module analysis supports it. Check the generated bundle when size matters; the import spelling alone does not guarantee a particular size.

EffectScript's global names are source conveniences. They refer to Effect and language support code, which still participates in the emitted module graph.

## Reusable operators

```ts
function withinSecond<A, E, R>(program: () => Effect<A, E, R>) {
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

Skip imports for the prelude in `.es`. In `.ts` integration files, import `Effect`, `Layer`, and the other names you use as usual.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/importing-effect)
