import type { VibeLangOptions } from "./options.js"

/** A bundler whose plugin list awaits promises. */
export type DeferredBundler = "rolldown" | "rollup" | "vite"

/**
 * The vibelang plugin for `bundler`, loaded only when the bundler awaits it.
 * For a config evaluated where the bundle runs, such as an Alchemy Worker's
 * `build` props: the bundled program holds this small object, not the
 * compiler backend. It is a promise in shape: `then`, `catch`, and `finally`
 * start the import.
 */
export declare function deferred(bundler: DeferredBundler, options?: VibeLangOptions): Promise<DeferredPlugin>

/**
 * The plugin a deferred value resolves to, typed by its name alone so that it
 * fits every bundler version's plugin type.
 */
export interface DeferredPlugin {
  readonly name: string
}
export default deferred
