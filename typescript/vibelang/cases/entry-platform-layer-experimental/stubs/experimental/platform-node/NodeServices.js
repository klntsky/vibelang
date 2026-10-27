import * as Effect from "effect/Effect"
import * as Layer from "effect/Layer"
import { Process } from "../Process.js"

// A Process whose pid says the experimental bundle provided it.
export const layer = Layer.succeed(Process, { pid: Effect.succeed(42) })
