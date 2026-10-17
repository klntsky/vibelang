// TypeScript code that imports the .effeck services through their declarations.
import { Effect, Layer } from "effect"
import { Flags, greet, Users } from "../../out/main.js"

class UsersFake implements Users {
  readonly source = "fake"
  find(id: string) {
    return Effect.succeed({ id, name: `user ${id}` })
  }
}

const program = Effect.gen(function* () {
  const users = yield* Users
  const user = yield* users.find("7")
  const flags = yield* Flags
  return `${user.name} from ${users.source}, beta ${flags.beta}`
})

export const fake: Effect.Effect<string, never, never> = Effect.provide(program, Layer.succeed(Users, new UsersFake()))
export const live: Effect.Effect<string, never, never> = Effect.provide(greet("1"), Users.layer)
