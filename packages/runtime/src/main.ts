import { Runtime } from "effect"

// The part of a host's process object that the runner uses: Node.js, Bun,
// and Deno provide it; a browser does not.
interface Host {
  readonly on: (signal: string, listener: () => void) => unknown
  readonly removeListener: (signal: string, listener: () => void) => unknown
  readonly exit: (code: number) => never
}

/**
 * D135: runs an entry module's main, `if (import.meta.main) $runMain(main())`,
 * when `@effect/platform-node` does not resolve from the module. It behaves as
 * `NodeRuntime.runMain`: Effect's runner reports a failure nothing reported,
 * SIGINT and SIGTERM interrupt the program when the host has a process, and
 * the exit code follows `Runtime.defaultTeardown`.
 */
export const $runMain = Runtime.makeRunMain(({ fiber, teardown }) => {
  const host = (globalThis as { readonly process?: Host }).process
  let receivedSignal = false
  const onSignal = () => {
    receivedSignal = true
    fiber.interruptUnsafe(fiber.id)
  }
  fiber.addObserver((exit) => {
    host?.removeListener("SIGINT", onSignal)
    host?.removeListener("SIGTERM", onSignal)
    teardown(exit, (code) => {
      if (host !== undefined && (receivedSignal || code !== 0)) host.exit(code)
    })
  })
  host?.on("SIGINT", onSignal)
  host?.on("SIGTERM", onSignal)
})
