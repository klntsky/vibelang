import { Runtime } from "effect"

// The part of a host's process object that the runner uses: Node.js, Bun,
// and Deno provide it; a browser does not.
interface Host {
  readonly on: (signal: string, listener: () => void) => unknown
  readonly removeListener: (signal: string, listener: () => void) => unknown
  readonly exit: (code: number) => never
}

// The part of a host's process object that $entry uses to find the module
// the host started: Node.js and Bun provide it.
interface EntryHost {
  readonly argv?: ReadonlyArray<string>
  // The CommonJS module the host started; undefined when it started an ES module.
  readonly mainModule?: { readonly exports?: { readonly main?: unknown } }
  readonly getBuiltinModule?: (id: string) => unknown
}

interface NodeUrl { readonly pathToFileURL: (path: string) => URL }
interface NodeFs { readonly realpathSync: (path: string) => string }

// The file URLs of the script the host started, as given and with its
// symbolic links resolved, since hosts name their main module either way; or
// none when the host does not say.
const startedFiles = (host: EntryHost | undefined): ReadonlyArray<string> => {
  const script = host?.argv?.[1]
  const url = host?.getBuiltinModule?.("node:url") as NodeUrl | undefined
  const fs = host?.getBuiltinModule?.("node:fs") as NodeFs | undefined
  if (typeof script !== "string" || url === undefined || fs === undefined) return []
  try {
    return [url.pathToFileURL(script).href, url.pathToFileURL(fs.realpathSync(script)).href]
  } catch {
    return []
  }
}

/**
 * D135: runs an entry module's main when the host started the module. The
 * module ends with
 * `$entry(import.meta.main, import.meta.url, main, () => NodeRuntime.runMain(main()))`.
 *
 * - `started` false: the module was imported; nothing runs.
 * - `started` true: the host started this module, or a bundle that holds it.
 *   Bun, webpack, and Rspack set import.meta.main for each bundled module;
 *   esbuild, Rollup, Rolldown, and Vite keep it as it is in an ES module
 *   bundle, where every module's is the bundle's. So when `url` names the
 *   file the host started, $entry imports that file, which the host has
 *   already loaded, and runs only the main it exports: the bundle's entry
 *   module's, or the module's own when it is not bundled. When it names
 *   another file, a bundler wrote each module's own URL and set `started`
 *   for it, and main runs.
 * - `started` undefined: a CommonJS bundle, which empties import.meta, or a
 *   file emitted as CommonJS, which passes undefined. Once the host's main
 *   module has run, main runs when that module exports it.
 */
export const $entry = (started: boolean | undefined, url: string | undefined, main: unknown, run: () => void): void => {
  const host = (globalThis as { readonly process?: EntryHost }).process
  if (started === true) {
    if (url === undefined || !startedFiles(host).includes(url)) return run()
    import(/* webpackIgnore: true */ /* @vite-ignore */ url).then(
      (module: { readonly main?: unknown }) => {
        if (module.main === main) run()
      },
      () => run()
    )
  } else if (started === undefined) {
    const module = host?.mainModule
    if (module !== undefined) {
      queueMicrotask(() => {
        if (module.exports?.main === main) run()
      })
    }
  }
}

/**
 * D135: the runner of an entry module's main,
 * `$entry(import.meta.main, import.meta.url, main, () => $runMain(main()))`,
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
  // addObserver runs immediately when main already completed synchronously.
  host?.on("SIGINT", onSignal)
  host?.on("SIGTERM", onSignal)
  fiber.addObserver((exit) => {
    host?.removeListener("SIGINT", onSignal)
    host?.removeListener("SIGTERM", onSignal)
    teardown(exit, (code) => {
      if (host !== undefined && (receivedSignal || code !== 0)) host.exit(code)
    })
  })
})
