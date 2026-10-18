import type { UnpluginInstance } from "unplugin"

export interface EffeckOptions {
  /** tsconfig path relative to the root. Defaults to tsconfig.json. */
  project?: string
  /**
   * Project root. The compiler runs here with rootDir set to it.
   * Defaults to the Vite root, the webpack or Rspack context, esbuild's
   * absWorkingDir, or process.cwd().
   */
  root?: string
  /** Native effeck binary or Node launcher path. Defaults to EFFECK_BINARY, then the effeck package. */
  compiler?: string
}

export declare const unplugin: UnpluginInstance<EffeckOptions | undefined, false>
export default unplugin
