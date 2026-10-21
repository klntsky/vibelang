# Running Effects

Runners start a program at an application boundary. Inside a running VibeLang function, calls already run on the current fiber, so reserve runners for entry points and framework adapters.

Call a runner at module top level, or in a plain callback registered there with a TypeScript function, such as a `describe` body or framework route handler. A runner inside code a fiber runs is EK1005; call the program or use `fork` instead. Tests in `.vibe` use `it.effect` from `@effect/vitest`; Promise-style tests stay in `.ts`.

An explicitly Effect-typed top-level `const` stores a cold program (D262):

```vibe
export const migrate: Effect.Effect<void, never, never> = Effect.suspend(() => {
  Console.log("migrating")
})
```

Importing this module prints nothing. To execute it, call `migrate()` inside a running function or pass `migrate` to a runner. Without the declared Effect type, the top-level call still reports EK1001; a `let`, a non-Effect annotation, or a type assertion does not opt in. Operands follow the ordinary by-name rules, so an eager program call inside the initializer needs its own program position.

An object-literal field also holds a cold program when its contextual type is an Effect, whether that type comes from a service, a parameter, or a declaration:

```vibe
interface Journal { read: Effect.Effect<number, never, never> }
const makeNoop = (): Journal => ({ read: Effect.sync(() => 0) })
```

Constructing the object runs nothing; calling `journal.read()` inside a running function executes the stored program. The same holds in `Layer.sync` service literals and in `make({ read: work() })` when `make` expects an Effect field. A field typed `number` instead runs `work()` during construction and stores its result.

## Build and run a local project

After [installing and scaffolding](004-installation.md), use the starter scripts:

```sh
bun run build
bun run start
bun run dev
bun run test
```

`build` runs `vibelang` with the project's `tsconfig.json`. `start` builds, then runs `node --enable-source-maps dist/main.js`. `dev` builds once, then runs `vibelang --watch` beside `node --watch --enable-source-maps dist/main.js`. `test` runs Vitest on `src/**/*.test.vibe` through `vibelang-vite`, which compiles the project with `vibelang` and serves the JavaScript, so tests need no prior build. Node and Vitest receive JavaScript; neither interprets `.vibe` sources.

The starter's `main.vibe` exports `main`, so `node dist/main.js` runs it with `NodeRuntime.runMain`, the application boundary. Its greeting and recovery messages quickly confirm that the service layer and typed failure both run.

## Execute synchronous computations

```vibe
// run: Console.log(total)
function subtotal() {
  return 7 * 8
}

const total = Effect.runSync(subtotal())
// total: number, 56
// Output: 56
```

`Effect.runSync` starts the program immediately and returns its result; an unhandled failure or defect escapes as a JavaScript exception at this boundary. If execution reaches asynchronous work, the runner throws `AsyncFiberError`, so use it only when the whole computation is known to stay synchronous.

## Run promise-based computations

```vibe
// run: Console.log(sync((value: Promise<string>) => value, pending))
function preparedMessage() {
  sleep("15 millis")
  return "prepared"
}

const pending = Effect.runPromise(preparedMessage())
// pending: Promise<string>
// Output: prepared
```

`Effect.runPromise` starts execution and returns a promise for the success value, which rejects when the program fails. A foreign TypeScript caller can await that promise; a VibeLang caller calls `preparedMessage()` directly inside its own program.

## Collect complete execution results

```vibe
// run: Console.log(immediate._tag)
error BadInput

function validate(input: string) {
  if (input === "") throw new BadInput()
  return input.length
}

const immediate = Effect.runSyncExit(() => validate(""))
const eventual = Effect.runPromiseExit(preparedMessage())
// immediate: Exit<number, BadInput>
// eventual: Promise<Exit<string, never>>
// Output: Failure
```

An `Exit` is a success with a value or a failure with a `Cause`, which keeps expected failures, defects, and interruption instead of reducing them to an ordinary error value. `runSyncExit` reports asynchronous execution as a defect in its failure result; `runPromiseExit` supports work that pauses.

## Start and interrupt a fiber

```vibe
// run: Console.log(Exit.isFailure(Fiber.await(running)))
function heartbeat() {
  repeat(log("alive"), Schedule.spaced("200 millis"))
}

const running = Effect.runFork(Effect.provide(heartbeat(), Logger.layer([])))

Effect.runPromise {
  sleep("650 millis")
  running.interrupt()
}
// Output: true
```

The empty logger layer keeps this heartbeat quiet. `Effect.runFork` returns a running `Fiber` to join, observe, or interrupt; `Fiber.interrupt` waits for its cleanup. Here a second runner acts as an external owner; in application logic, use block-owned `fork(() => heartbeat())` and let the enclosing block bound its lifetime.

## Select an application boundary

```vibe
// main.vibe
export function main() {
  Console.log("application started")
}
// node main.js runs main with NodeRuntime.runMain
// Output: application started
```

When node starts a module that exports a `main` without parameters, the platform runner runs it, so shutdown follows platform conventions; without `@effect/platform-node` installed, the compiler uses the runtime's copy of that runner. To pass options or layers, call a runner at the top level yourself; the module then runs only that. Supply the application layer once around `main`: a runner needs all application requirements satisfied. For several framework entry points sharing services, use [ManagedRuntime](../036-runtime.md).

| Runner | Result |
| --- | --- |
| `Effect.runSync` | `A` |
| `Effect.runSyncExit` | `Exit<A, E>` |
| `Effect.runPromise` | `Promise<A>` |
| `Effect.runPromiseExit` | `Promise<Exit<A, E>>` |
| `Effect.runFork` | `Fiber<A, E>` |

The three contract channels do not say whether work runs synchronously. Prefer an asynchronous runner unless the host requires a synchronous answer and the operation can give one.

## Coming from Effect

In `.vibe` files, pass `main()` too: the runner receives the call by name, as a program; `() => main()` means the same. A TypeScript caller gets a cold Effect from an exported VibeLang function and passes `main()` to the runner the same way.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/running-effects)
