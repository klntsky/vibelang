// TypeScript side of Chapter 10 (and of ch02-plain-ts-call.fail.vibe): an Effect.fn function, a
// TypeScript-authored Context.Service, plain functions, and a Promise-returning function.
import { Context, Data, Effect, Layer } from "effect"

export interface User {
  readonly id: string
  readonly name: string
  readonly email: string
}

export interface Row {
  readonly id: string
  readonly name: string
  readonly email: string
}

export interface Avatar {
  readonly userId: string
  readonly url: string
}

export class UserNotFound extends Data.TaggedError("UserNotFound")<{ readonly id: string }> {}
export class DbError extends Data.TaggedError("DbError")<{ readonly reason: string }> {}
export class AvatarError extends Data.TaggedError("AvatarError")<{ readonly id: string }> {}

export class Database extends Context.Service<Database, {
  readonly query: (sql: string, params: ReadonlyArray<string>) => Effect.Effect<ReadonlyArray<Row>, DbError>
}>()("examples/ch10-helpers/Database") {}

export class Storage extends Context.Service<Storage, {
  readonly get: (key: string) => Effect.Effect<string | undefined>
}>()("examples/ch10-helpers/Storage") {}

const toUser = (row: Row): User => ({ id: row.id, name: row.name, email: row.email })

// users-repo.ts from Chapter 10.
// (id: string) => Effect<User, UserNotFound | DbError, Database>
export const findUser = Effect.fn("findUser")(function*(id: string) {
  const db = yield* Database
  const rows = yield* db.query("select * from users where id = $1", [id])
  if (rows.length === 0) return yield* new UserNotFound({ id })
  return toUser(rows[0])
})

// A plain TypeScript function: (user: User) => string.
export const formatName = (user: User): string => `${user.name} <${user.email}>`

// A plain TypeScript function that throws on bad input.
export const parsePort = (text: string): number => {
  const port = Number(text)
  if (!Number.isInteger(port)) throw new Error(`not a port: ${text}`)
  return port
}

export class PortError extends Data.TaggedError("PortError")<{ readonly text: string }> {}

/**
 * The same parser with a trusted contract (D23): .vibe files may call it bare, and PortError joins E.
 * @vibelang trusted
 * @throws {PortError} when the text is not an integer
 */
export const parsePortTrusted = (text: string): number => {
  const port = Number(text)
  if (!Number.isInteger(port)) throw new PortError({ text })
  return port
}

// (id: string) => Effect<Avatar, AvatarError, Storage>
export const loadAvatar = (id: string): Effect.Effect<Avatar, AvatarError, Storage> =>
  Effect.gen(function*() {
    const storage = yield* Storage
    const url = yield* storage.get(`avatar/${id}`)
    if (url === undefined) return yield* new AvatarError({ id })
    return { userId: id, url }
  })

// Stands in for fetch, which .vibe files may not call (D5: EK4402).
export const fetchGreeting = (name: string): Promise<{ readonly greeting: string }> =>
  Promise.resolve({ greeting: `hello ${name}` })

export const DatabaseTest = Layer.succeed(Database, {
  query: (_sql, params) =>
    Effect.succeed(params[0] === "1" ? [{ id: "1", name: "ada", email: "ada@example.com" }] : [])
})

export const StorageTest = Layer.succeed(Storage, {
  get: (key) => Effect.succeed(key === "avatar/1" ? "https://img.example.com/1.png" : undefined)
})
