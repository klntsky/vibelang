import type { Plugin } from "vite"

export interface EffeckOptions {
  /** tsconfig path relative to Vite's root. Defaults to tsconfig.json. */
  project?: string
  /** Native effeck binary or Node launcher path. Defaults to effeck. */
  compiler?: string
}

export default function effeck(options?: EffeckOptions): Plugin
