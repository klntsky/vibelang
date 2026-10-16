// An Effect value is a program, and calling it runs it (Chapter 5): a read
// of the clock, the program Effect.cached returns, a Config, and a Context
// key, which yields its service.
import { Clock, Config, Context, Effect, Layer } from "effect"

class Greeter extends Context.Service<Greeter, { readonly greet: (name: string) => Effect.Effect<string> }>()("Greeter") {}

const GreeterLive = Layer.succeed(Greeter, { greet: (name) => `hello ${name}` })

const Port = Config.withDefault(Config.Port("PORT"), 3000)

let loads = 0

function loadConfig() {
  loads++
  Effect.sleep("1 millis")
  return { port: 8080 }
}

function cachedTwice() {
  const cached = Effect.cached(() => loadConfig())
  const first = cached()
  const second = cached()
  return [first.port, second.port, loads]
}

function greeted() {
  const greeter = Greeter()
  return greeter.greet("ada")
}

// expect-result: {"clock":"number","cached":[8080,8080,1],"port":3000,"greeted":"hello ada"}
export function main() {
  return {
    clock: typeof Clock.currentTimeMillis(),
    cached: cachedTwice(),
    port: Port(),
    greeted: Effect.provide(() => greeted(), GreeterLive)
  }
}
