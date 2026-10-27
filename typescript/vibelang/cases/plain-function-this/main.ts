import { Effect } from "effect"
import { read } from "./read.vibe"

console.log(Effect.runSync(read.call({ value: 42 })))
