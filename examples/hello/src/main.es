import { Console } from "effect"
import { NodeRuntime } from "@effect/platform-node"

export function main() {
  Console.log("Hello, EffectScript!")
}

NodeRuntime.runMain(() => main())
