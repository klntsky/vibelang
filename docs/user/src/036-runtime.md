# Introduction to Runtime

Effect's runtime executes computations, carries service implementations, schedules fibers, and runs cleanup. Effeck uses that infrastructure while ordinary functions express the work to execute.

## Execution infrastructure

```ts
function operation() {
  Effect.sleep("10 millis")
  return "complete"
}

const outcome = Effect.runPromiseExit(() => operation())
```

A runner starts a root fiber with its context, fiber-local settings, and initial program. The runtime steps through the work, waits for asynchronous results, coordinates interruption, and executes finalizers. `outcome` is a promise for an `Exit` that records either the success or the complete failure cause.

A function value remains a description until it is called or handed to a runner. Calling another Effeck function inside `operation` continues on the same fiber; concurrency requires an explicit operator.

## Run with an explicit service context

```ts
abstract class Labels {
  abstract format(value: string): Effect<string, never, never>
}

const context = Context.make(Labels, {
  format: (value) => `[${value}]`
})

function label() {
  return Labels.format("queued")
}

const result = Effect.runSyncWith(context)(() => label())
```

`Context.make` associates a service key with its implementation. The `run*With` variants accept that context and execute a program whose requirements it satisfies. `runPromiseWith` and `runForkWith` serve asynchronous and fiber-based entry points; plain runners require no remaining application services.

Call runners only where no fiber runs the code: at a module's top level or in a plain callback registered there with a TypeScript host. Calling one inside a running program is EK1005; call the program directly or use `fork` there.

Most applications build their context through layers instead. Use an explicit context when the host already has one, or a managed runtime when several entry points must share layer resources.

## Change runtime behavior within a region

```ts
const Quiet = Logger.layer([])

function startup() {
  Effect.log("visible before")
  Effect.provide(() => {
    Effect.log("suppressed in this region")
  }, Quiet)
  Effect.log("visible after")
}
```

`Effect.provide` installs the layer only around its program. The previous logger configuration is restored afterward, including on failure. Nested regions can override it again, and child fibers inherit the configuration in effect when they start.

To emit a different format, supply a logger such as `Logger.consoleJson` through `Logger.layer([Logger.consoleJson])`. This uses Effect's console integration instead of a browser or Node global.

## Build and dispose managed runtimes

```ts
// notifications.effeck
export abstract class Notifications {
  abstract send(message: string): Effect<void, never, never>

  static readonly layer = Layer.succeed(Notifications, {
    send: (message) => Console.log(message)
  })
}

export function notify(message: string) {
  Notifications.send(message)
}
```

```ts
// TypeScript (.ts)
// host.ts
import { ManagedRuntime } from "effect"
import { Notifications, notify } from "./notifications.js"

const runtime = ManagedRuntime.make(Notifications.layer)

export async function example() {
  try {
    await runtime.runPromise(notify("first event"))
    await runtime.runPromise(notify("second event"))
  } finally {
    await runtime.dispose()
  }
}
```

`ManagedRuntime.make` builds its layer lazily on first use and shares the resulting context across executions. The layer's resources live until disposal rather than until each call completes. Dispose the runtime when its owner shuts down, and create a fresh runtime for a new owner.

Each `Effect.provide` builds its layers once for that invocation. Repeating `provide` at several entry points rebuilds them, which is why a managed runtime is useful for shared pools or clients. For Effect-based cleanup, `runtime.disposeEffect` is an Effect value that runs when called from `.effeck`.

## Connect runtime execution to frameworks

```ts
// TypeScript (.ts)
// notification-host.ts
import { ManagedRuntime } from "effect"
import { Notifications, notify } from "./notifications.js"

export function makeNotificationHost() {
  const runtime = ManagedRuntime.make(Notifications.layer)
  return {
    onMessage: (message: string) => runtime.runPromise(notify(message)),
    close: () => runtime.dispose()
  }
}
```

The framework owns this adapter: call `onMessage` at an entry point and `close` when the application, test fixture, or component owner ends. Its promise reports failures back to the host. Keep the runtime stable across repeated requests or renders so layer resources are shared as intended.

Framework callbacks can stay in TypeScript and import the emitted Effeck functions, as above. A plain framework callback registered from an `.effeck` module's top level can also call `runtime.runPromise(() => notify(message))`. The callback runs outside a fiber; the runner supplies the boundary to the application program.

## Coming from Effect

Abstract service classes replace hand-written `Context.Service` declarations in `.effeck`. The runtime, context, layers, and managed runtime remain Effect's APIs; TypeScript boundary code continues to pass cold Effects to their runners.

[Upstream Effect page](https://effect.website/docs/v4/runtime)
