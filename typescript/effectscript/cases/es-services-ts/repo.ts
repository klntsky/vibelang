// A service written in TypeScript as Effect 4 declares one.
import { Context, Data, Effect, Layer } from "effect"

export interface User {
  readonly id: string
  readonly name: string
}

export class UserNotFound extends Data.TaggedError("UserNotFound")<{ readonly id: string }> {}

export class UserRepo extends Context.Service<UserRepo, {
  readonly byId: (id: string) => Effect.Effect<User, UserNotFound>
  readonly all: Effect.Effect<ReadonlyArray<User>>
  readonly pipe: (label: string) => Effect.Effect<string>
}>()("es-services-ts/UserRepo") {}

const users: ReadonlyArray<User> = [{ id: "1", name: "Ada" }, { id: "2", name: "Grace" }]

export const UserRepoLive = Layer.succeed(UserRepo, {
  byId: (id) => {
    const user = users.find((u) => u.id === id)
    return user === undefined ? Effect.fail(new UserNotFound({ id })) : Effect.succeed(user)
  },
  all: Effect.succeed(users),
  pipe: (label) => Effect.succeed(`piped ${label}`)
})
