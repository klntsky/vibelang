import { Context, Data, Effect, Layer } from "effect"

class Bad extends Data.TaggedError("Bad")<{ readonly code: string }> {}

class Users extends Context.Service<Users, {
  readonly find: (id: string) => Effect.Effect<string, Bad>
}>()("Users") {}

export const UsersLive = Layer.succeed(Users, {
  find: Effect.fnUntraced(function* (id: string) {
    if (id !== "42") return yield* Effect.fail(new Bad({ code: id }))
    return "Ada"
  })
})

export const nameOf = Effect.fnUntraced(function* (id: string) {
  const users = yield* Users
  return yield* Effect.orElseSucceed(users.find(id), () => "missing")
})

export const greet = Effect.fnUntraced(function* (id: string) {
  const users = yield* Users
  const name = yield* users.find(id)
  return `hi ${name}`
})

export const neverGreeted = Effect.fnUntraced(function* (id: string) {
  const users = yield* Users
  const name = yield* users.find(id)
  return `bye ${name}`
})
