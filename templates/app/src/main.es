import { Console } from "effect"
import { NodeRuntime } from "@effect/platform-node"
import { EmptyName, greeting } from "./greeting.es"

abstract class Greeter {
  abstract greet(name: string): Effect<string, EmptyName, never>
}

const GreeterLive = Layer.succeed(Greeter, {
  greet: (name: string) => greeting(name)
})

export function main() {
  Effect.provide(() => {
    Console.log(Greeter.greet("EffectScript"))
    try {
      Greeter.greet("")
    } catch (error) {
      Console.log(`Recovered from ${error._tag}`)
    }
  }, GreeterLive)
}

NodeRuntime.runMain(() => main())
