import type { Plugin } from "vite"

export interface EffectScriptOptions {
  /** tsconfig path relative to Vite's root. Defaults to tsconfig.json. */
  project?: string
  /** Native esc binary or Node launcher path. Defaults to @effectscript/compiler. */
  compiler?: string
}

export default function effectscript(options?: EffectScriptOptions): Plugin
