// Reference program 1, TypeScript side: an existing Effect service with a generic, an overloaded, and
// an aliased Effect function, plus a TypeScript-authored Context.Service class and its Layer.
import { Context, Data, Effect, Layer } from "effect"

export interface User {
  readonly id: string
  readonly name: string
  readonly team: string
}

export class UserNotFound extends Data.TaggedError("UserNotFound")<{ readonly id: string }> {}
export class DbError extends Data.TaggedError("DbError")<{ readonly reason: string }> {}

export class UserRepo extends Context.Service<UserRepo, {
  readonly byId: (id: string) => Effect.Effect<User, UserNotFound | DbError>
  readonly all: Effect.Effect<ReadonlyArray<User>, DbError>
}>()("examples/ref1-users-repo/UserRepo") {}

const users: ReadonlyArray<User> = [
  { id: "1", name: "Ada", team: "core" },
  { id: "2", name: "Grace", team: "compilers" },
  { id: "3", name: "Edsger", team: "core" }
]

// The Layer is provided from TypeScript.
export const UserRepoLive = Layer.succeed(UserRepo, {
  byId: (id) => {
    const user = users.find((u) => u.id === id)
    return user === undefined ? Effect.fail(new UserNotFound({ id })) : Effect.succeed(user)
  },
  all: Effect.succeed(users)
})

// Generic: the field and its value type are type parameters.
export const findBy = <K extends keyof User>(
  key: K,
  value: User[K]
): Effect.Effect<User, UserNotFound | DbError, UserRepo> =>
  Effect.gen(function*() {
    const repo = yield* UserRepo
    const all = yield* repo.all
    const user = all.find((u) => u[key] === value)
    if (user === undefined) return yield* new UserNotFound({ id: `${String(key)}=${String(value)}` })
    return user
  })

// Overloaded: one id or many.
export function lookup(id: string): Effect.Effect<User, UserNotFound | DbError, UserRepo>
export function lookup(ids: ReadonlyArray<string>): Effect.Effect<ReadonlyArray<User>, UserNotFound | DbError, UserRepo>
export function lookup(
  arg: string | ReadonlyArray<string>
): Effect.Effect<User | ReadonlyArray<User>, UserNotFound | DbError, UserRepo> {
  return Effect.gen(function*() {
    const repo = yield* UserRepo
    if (typeof arg === "string") return yield* repo.byId(arg)
    const out: User[] = []
    for (const id of arg) out.push(yield* repo.byId(id))
    return out
  })
}

// Aliased: one Effect.fn function under three names, through a const, a type alias, and a re-export.
const fetchUserImpl = Effect.fn("fetchUser")(function*(id: string) {
  const repo = yield* UserRepo
  return yield* repo.byId(id)
})
export const fetchUser = fetchUserImpl
export type FetchUser = typeof fetchUserImpl
export const getUser: FetchUser = fetchUserImpl
export { fetchUserImpl as loadUser }
