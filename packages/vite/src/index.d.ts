import type { Plugin } from "vite"

export interface VibeLangOptions {
  /** tsconfig path relative to Vite's root. Defaults to tsconfig.json. */
  project?: string
  /** Native vibelang binary or Node launcher path. Defaults to vibelang. */
  compiler?: string
}

export default function vibelang(options?: VibeLangOptions): Plugin
