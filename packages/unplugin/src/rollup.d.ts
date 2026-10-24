import type { Plugin } from "rollup"

import type { VibeLangOptions } from "./options.js"

declare const vibelang: (options?: VibeLangOptions) => Plugin
export default vibelang
