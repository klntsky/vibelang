# Running Effects

Runners start a program at an application boundary. Inside a running EffectScript function, ordinary calls already run computations on the current fiber, so reserve runners for entry points and framework adapters.

Call a runner at module top level or in a plain callback registered there with a TypeScript function, such as a `describe` body or framework route handler. Calling a runner inside code a fiber runs is ES1005; call the program or use `fork` instead. Tests in `.es` use `it.effect` from `@effect/vitest`; Promise-style tests stay in `.ts`.

## Execute synchronous computations

```ts
function subtotal() {
  return 7 * 8
}

const total = Effect.runSync(() => subtotal())
// total: number, 56
```

`Effect.runSync` starts the program immediately and returns its result. An unhandled failure or defect escapes as a JavaScript exception at this boundary. If execution reaches asynchronous work, the runner throws `AsyncFiberError`; use it only when the whole computation is known to stay synchronous.

## Run promise-based computations

```ts
function preparedMessage() {
  Effect.sleep("15 millis")
  return "prepared"
}

const pending = Effect.runPromise(() => preparedMessage())
// pending: Promise<string>
```

`Effect.runPromise` starts execution and returns a promise for the success value. The promise rejects when the program fails. A foreign TypeScript caller can await that promise; an EffectScript caller should call `preparedMessage()` directly inside its own program.

## Collect complete execution results

```ts
class BadInput extends TaggedError("BadInput")<{}> {}

function validate(input: string) {
  if (input === "") throw new BadInput()
  return input.length
}

const immediate = Effect.runSyncExit(() => validate(""))
const eventual = Effect.runPromiseExit(() => preparedMessage())
// immediate: Exit<number, BadInput>
// eventual: Promise<Exit<string, never>>
```

An `Exit` is either a success with a value or a failure with a `Cause`. That cause retains expected failures, defects, and interruption rather than reducing them to an ordinary error value. `runSyncExit` reports asynchronous execution as a defect in its failure result; `runPromiseExit` supports work that pauses.

## Start and interrupt a fiber

```ts
import { Fiber, Schedule } from "effect"

function heartbeat() {
  Effect.repeat(() => Effect.log("alive"), Schedule.spaced("200 millis"))
}

const running = Effect.runFork(() => heartbeat())

Effect.runPromise(() => {
  Effect.sleep("650 millis")
  Fiber.interrupt(running)
})
```

`Effect.runFork` returns a running `Fiber`, which you can join, observe, or interrupt. `Fiber.interrupt` waits for its cleanup. This example uses a second runner to act as an external owner; inside application logic, use block-owned `fork(() => heartbeat())` and let the enclosing block bound its lifetime.

## Select an application boundary

```ts
// main.es
import { NodeRuntime } from "@effect/platform-node"

function main() {
  Effect.log("application started")
}

NodeRuntime.runMain(() => main())
```

Use the platform runner for a command-line application so shutdown follows platform conventions. Supply the application layer once around `main`; a runner needs all application requirements satisfied. For multiple framework entry points sharing services, use [ManagedRuntime](../036-runtime.md).

| Runner | Result |
| --- | --- |
| `Effect.runSync` | `A` |
| `Effect.runSyncExit` | `Exit<A, E>` |
| `Effect.runPromise` | `Promise<A>` |
| `Effect.runPromiseExit` | `Promise<Exit<A, E>>` |
| `Effect.runFork` | `Fiber<A, E>` |

The three contract channels do not say whether work runs synchronously. Prefer an asynchronous runner unless the host requires a synchronous answer and the operation satisfies that requirement.

## Coming from Effect

Pass `() => main()` in `.es` files. A TypeScript caller receives a cold Effect from an exported EffectScript function and passes `main()` to the runner instead.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/running-effects)
