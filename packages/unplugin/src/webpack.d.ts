import type { WebpackPluginInstance } from "webpack"

import type { VibeLangOptions } from "./options.js"

declare const vibelang: (options?: VibeLangOptions) => WebpackPluginInstance
export default vibelang
