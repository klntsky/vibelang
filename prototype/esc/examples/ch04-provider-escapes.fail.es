// expect: ES2001
// Chapter 4 "Writing a provider": without Effect.orDie, DbError escapes UsersLive.find, but
// Users.find's contract allows only UserNotFound.

class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}
class DbError extends TaggedError("DbError")<{ readonly reason: string }> {}

interface User {
  readonly id: string
  readonly name: string
  readonly email: string
}

interface Row {
  readonly id: string
  readonly name: string
  readonly email: string
}

abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
  abstract rename(id: string, name: string): Effect<void, UserNotFound, never>
}

abstract class Database {
  abstract query(sql: string, params: ReadonlyArray<string>): Effect<ReadonlyArray<Row>, DbError, never>
  abstract execute(sql: string, params: ReadonlyArray<string>): Effect<void, DbError, never>
}

function toUser(row: Row) {
  const user: User = { id: row.id, name: row.name, email: row.email }
  return user
}

const UsersLive = Layer.effect(Users, () => {
  const db = Database()
  return {
    find(id) {
      const rows = db.query("select * from users where id = $1", [id])
      if (rows.length === 0) throw new UserNotFound({ id })
      return toUser(rows[0])
    },
    rename(id, name) {
      Effect.orDie(() => db.execute("update users set name = $1 where id = $2", [name, id]))
    }
  }
})
