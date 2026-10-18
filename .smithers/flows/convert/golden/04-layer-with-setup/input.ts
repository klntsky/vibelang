import { Context, Effect, Layer } from "effect"

class Greeter extends Context.Service<Greeter, {
  greet(name: string): Effect.Effect<string>
}>()("app/Greeter") {}

export const GreeterLive = Layer.effect(Greeter, Effect.gen(function* () {
  const prefix = "Hello"
  yield* Effect.log("greeter ready")
  return Greeter.of({ greet: (name) => Effect.succeed(`${prefix}, ${name}`) })
}))
