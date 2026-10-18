import { Effect } from "effect"
export const marker = new Error("workspace defect")
export function data() { return 42 }
export function explode(): number { throw marker }
export function program(): Effect.Effect<number, never, never> { return Effect.succeed(7) }
