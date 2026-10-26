import { Effect } from "effect"
import { hello } from "./hello.vibe"

// tsc writes this import as ./hello.js; vibelang-mapper --emit writes hello.js.
console.log(Effect.runSync(hello("Ada")))
