// expect: ES3101
// A member that looks a service up each time it is called needs that
// service in the caller's Context, so the contract must declare it (Chapter
// 4, "Requirements that would leak"). Capturing it when the layer is built,
// as Database is here, is the other fix.
import { Data, Effect, Layer } from "effect"

class UserNotFound extends Data.TaggedError("UserNotFound")<{ readonly id: string }> {}

interface User {
  readonly id: string
  readonly name: string
}

abstract class Users {
  abstract find(id: string): Effect.Effect<User, UserNotFound, never>
}

abstract class Database {
  abstract query(sql: string): Effect.Effect<User | undefined, never, never>
}

abstract class Audit {
  abstract record(entry: string): Effect.Effect<void, never, never>
}

export const UsersAudited = Layer.effect(Users, () => {
  const db = Database()
  return {
    find(id) {
      Audit.record(`users.find ${id}`)
      const user = db.query(`select ${id}`)
      if (user === undefined) return Effect.fail(new UserNotFound({ id }))
      return Effect.succeed(user)
    }
  }
})

export class UsersLogged implements Users {
  find(id: string) {
    Audit.record(`find ${id}`)
    return Effect.fail(new UserNotFound({ id }))
  }
}
