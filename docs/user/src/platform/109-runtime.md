# Runtime

A platform `runMain` starts a VibeLang application and connects its lifetime to the host process. It reports unhandled failures, handles termination signals, waits for cleanup, and chooses an exit code.

## Running an application entry point

```vibe
// preparation.vibe
import { NodeRuntime } from "@effect/platform-node"

function main() {
  Console.log("Preparing export")
  sleep("100 millis")
  Console.log("Export complete")
}

NodeRuntime.runMain(() => main())
// Output: Preparing export
// Output: Export complete
```

Pass the runner a program: `main()` passed to a runner is passed by name, so it has not run yet. A module's top level can call the recognized runner, while the application work runs inside `main`. Supply application layers with `Effect.provide(() => main(), AppLive)` inside that runner's program.

Runners may also be called in plain callbacks passed from the top level to TypeScript, such as a framework's route handler or a `describe` body. A runner inside code that a fiber runs is EK1005: call the program or use `fork` there. Tests in `.vibe` use `it.effect`; Promise-style tests stay in `.ts`.

Bun applications use `BunRuntime.runMain` from `@effect/platform-bun`; Node.js and Deno use the Node adapter. Run the generated JavaScript entry module after the VibeLang build.

## Configuring runtime launch behavior

```vibe
// startup-rejected.vibe
import { NodeRuntime } from "@effect/platform-node"

error StartupRejected {
  reason: string
}

function main() {
  throw new StartupRejected({ reason: "configuration missing" })
}

NodeRuntime.runMain(() => main(), { disableErrorReporting: true })
```

`disableErrorReporting` suppresses automatic failure logging; it does not turn failure into success or change the default exit-code policy. Leave reporting enabled unless the application has another reporting boundary. The optional `teardown` callback controls completion handling and ultimately calls the provided exit callback.

```ts
// TypeScript (.ts)
// launch-options.ts
import { Exit, type Runtime } from "effect"

export const launchOptions = {
  teardown: ((exit, onExit) => {
    onExit(Exit.isSuccess(exit) ? 0 : 2)
  }) satisfies Runtime.Teardown
}
```

```vibe
// custom-entry.vibe
import { NodeRuntime } from "@effect/platform-node"
import { launchOptions } from "./launch-options.ts"

function main() {
  Console.log("Starting with custom exit handling")
}

NodeRuntime.runMain(() => main(), launchOptions)
// Output: Starting with custom exit handling
```

Teardown is a plain host callback, not an Effect-returning program. Keep a custom callback in TypeScript when it calls the host's plain `onExit` function; put effectful cleanup in the application before teardown runs. This example deliberately maps every failure to exit code 2.

## Handling completion and failures

```vibe
// invalid-batch.vibe
import { NodeRuntime } from "@effect/platform-node"

error InvalidBatch {
  count: number
}

function exportBatch(count: number) {
  try {
    if (count <= 0) throw new InvalidBatch({ count })
    Console.log(`Exporting ${count} records`)
  } finally {
    Console.log("Export cleanup finished")
  }
}

NodeRuntime.runMain(() => exportBatch(0))
```

Normal return is success; a value returned from `main` is not automatically printed. An unhandled expected failure or defect is reported by default and receives a failing exit code. Effect 4.0.5's default teardown uses 0 for success, 130 for interruption-only causes, and 1 for other failures unless a supported error-specific exit code overrides it.

On a termination signal, the platform interrupts the application and waits for registered cleanup. VibeLang `finally` and `using` run during interruption, while ordinary `catch` does not catch it. Handle recoverable failures inside the program, and let the runner report failures that remain unhandled.

## Check process completion

These host-side checks run the emitted entries in separate processes. They verify silent failure, cleanup before a failing exit, and both outcomes of the custom teardown.

```ts
// TypeScript (.ts)
// runtime-checks.ts
import type {} from "./startup-rejected.vibe"
import type {} from "./invalid-batch.vibe"
import type {} from "./custom-entry.vibe"
import { spawnSync } from "node:child_process"
import { fileURLToPath } from "node:url"

function launch(file: string) {
  return spawnSync(process.execPath, [fileURLToPath(new URL(file, import.meta.url))], { encoding: "utf8" })
}

const rejected = launch("./startup-rejected.js")
console.log(`rejected: ${rejected.status}, silent: ${rejected.stdout === "" && rejected.stderr === ""}`)
console.log(`custom: ${launch("./custom-entry.js").status}`)
const batch = launch("./invalid-batch.js")
console.log(`batch: ${batch.status}, cleaned: ${batch.stdout.includes("Export cleanup finished")}`)
const failedCustom = spawnSync(process.execPath, ["--input-type=module", "-e", `
  import { Effect } from "effect"
  import { NodeRuntime } from "@effect/platform-node"
  import { launchOptions } from "./launch-options.js"
  NodeRuntime.runMain(Effect.fail("rejected"), { ...launchOptions, disableErrorReporting: true })
`], { cwd: fileURLToPath(new URL(".", import.meta.url)), encoding: "utf8" })
console.log(`custom failure: ${failedCustom.status}`)
// Output: rejected: 1, silent: true
// Output: custom: 0
// Output: batch: 1, cleaned: true
// Output: custom failure: 2
```
