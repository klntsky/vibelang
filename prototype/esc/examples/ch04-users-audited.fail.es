// expect: ES3101
// Chapter 4 "Requirements that would leak": UsersAudited.find looks up Audit each time it is
// called, but Users.find's contract allows no call-time requirements.
// NOTE: findIn and renameIn are unspecified in the proposal; they take the captured Database
// implementation, typed with the service class name (D1: the class name is the service's shape).

class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}
class DbError extends TaggedError("DbError")<{ readonly reason: string }> {}

interface User {
  readonly id: string
  readonly name: string
  readonly email: string
}

abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
  abstract rename(id: string, name: string): Effect<void, UserNotFound, never>
}

abstract class Database {
  abstract query(sql: string, params: ReadonlyArray<string>): Effect<ReadonlyArray<User>, DbError, never>
  abstract execute(sql: string, params: ReadonlyArray<string>): Effect<void, DbError, never>
}

abstract class Audit {
  abstract record(entry: string): Effect<void, never, never>
}

function findIn(db: Database, id: string) {
  const rows = Effect.orDie(() => db.query("select * from users where id = $1", [id]))
  if (rows.length === 0) throw new UserNotFound({ id })
  return rows[0]
}

function renameIn(db: Database, id: string, name: string) {
  Effect.orDie(() => db.execute("update users set name = $1 where id = $2", [name, id]))
}

const UsersAudited = Layer.effect(Users, () => {
  const db = Database()
  return {
    find(id) {
      Audit.record(`users.find ${id}`)    // looked up on every call
      return findIn(db, id)
    },
    rename(id, name) { renameIn(db, id, name) }
  }
})
