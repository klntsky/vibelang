import { Effect } from "effect"
import { layer, Registry } from "./main.js"

export function check(): string {
  const config = { value: 1 }
  const built = layer(config)
  config.value = 2
  const program = Effect.provide(Registry, built)
  if (Effect.runSync(program).value !== 1 || Effect.runSync(program).value !== 1) {
    throw new Error("a layer must keep its call-time snapshot before each build")
  }
  return "layer snapshot: 1"
}
