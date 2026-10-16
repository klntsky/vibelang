// Services are abstract classes (D1, D45). A call to a member looks the
// service up and joins R (D41); Users() returns the implementation.
import { Data, Effect, Layer } from "effect"

class UserNotFound extends Data.TaggedError("UserNotFound")<{ readonly id: string }> {}

interface User {
  readonly id: string
  readonly name: string
}

abstract class Directory {
  abstract lookup(id: string): Effect.Effect<User | undefined, never, never>
}

abstract class Users {
  abstract find(id: string): Effect.Effect<User, UserNotFound, never>
}

abstract class Settings {
  abstract readonly greeting: string
  abstract shout(text: string): Effect.Effect<string, never, never>

  static readonly layer = Layer.succeed(Settings, {
    greeting: "Welcome",
    shout: (text: string) => Effect.succeed(`${text}!`)
  })
}

const users = new Map<string, User>([["1", { id: "1", name: "Ada" }]])

const DirectoryLive = Layer.succeed(Directory, {
  lookup: (id: string) => Effect.succeed(users.get(id))
})

// The provider captures Directory when the layer is built, so Directory is a
// requirement of the layer, not of Users.find.
function makeUsers() {
  const directory = Directory()
  const impl: Users = {
    find(id) {
      const user = directory.lookup(id)
      if (user === undefined) return Effect.fail(new UserNotFound({ id }))
      return Effect.succeed(user)
    }
  }
  return impl
}

const UsersLive = Layer.effect(Users, makeUsers())

// An implementation can be a class that implements the service.
class UsersInMemory implements Users {
  readonly rows = new Map<string, User>([["7", { id: "7", name: "Grace" }]])

  find(id: string) {
    const user = this.rows.get(id)
    if (user === undefined) return Effect.fail(new UserNotFound({ id }))
    return Effect.succeed(user)
  }
}

// A method that implements a service member compiles to an Effect even when
// its body runs nothing, in a class and in an object literal.
class UsersFake implements Users {
  find(id: string) {
    return { id, name: `fake ${id}` }
  }
}

const UsersGrace = Layer.succeed(Users, {
  find(id) {
    return { id, name: "Grace" }
  }
})

export function greet(id: string) {
  const user = Users.find(id)
  return Settings.shout(`${Settings().greeting}, ${user.name}`)
}

export function lookupBoth(id: string) {
  const first = Users().find(id)
  return first.name
}

const live = Effect.provide(greet("1"), Layer.mergeAll(Layer.provide(UsersLive, DirectoryLive), Settings.layer))
const memory = Effect.provide(greet("7"), Layer.mergeAll(Layer.succeed(Users, new UsersInMemory()), Settings.layer))
const direct = Effect.provide(lookupBoth("7"), Layer.succeed(Users, new UsersInMemory()))
const missing = Effect.flip(Effect.provide(greet("9"), Layer.mergeAll(Layer.succeed(Users, new UsersInMemory()), Settings.layer)))
const fake = Effect.provide(lookupBoth("3"), Layer.succeed(Users, new UsersFake()))
const grace = Effect.provide(lookupBoth("4"), UsersGrace)

// expect-result: {"live":"Welcome, Ada!","memory":"Welcome, Grace!","direct":"Grace","missing":"UserNotFound","fake":"fake 3","grace":"Grace"}
export function main() {
  return { live: live(), memory: memory(), direct: direct(), missing: missing()._tag, fake: fake(), grace: grace() }
}
