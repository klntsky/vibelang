import type { Effect } from "effect"
import type { Prompt } from "effect/cli"
import { plainBody, effectBody, noOperation } from "../out/main.js"
const first: Effect.Effect<Prompt.Prompt<string>, never, never> = plainBody(true)
const second: Effect.Effect<Prompt.Prompt<string>, never, never> = effectBody(false)
const number: Effect.Effect<number, never, never> = noOperation()
export { first, second, number }
