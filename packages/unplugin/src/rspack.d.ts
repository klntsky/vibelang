import type { RspackPluginInstance } from "@rspack/core"

import type { VibeLangOptions } from "./options.js"

declare const vibelang: (options?: VibeLangOptions) => RspackPluginInstance
export default vibelang
