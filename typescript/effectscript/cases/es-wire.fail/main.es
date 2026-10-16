// expect: ES3202 ES3203 ES3204
// wire rejects a graph it cannot order (D27): two providers for one service
// (ES3202), providers that need each other (ES3203), and an argument whose
// type does not name its outputs (ES3204).
import { Effect, Layer } from "effect"
import { wire } from "@effectscript/runtime"

abstract class Users {
  abstract find(id: string): Effect.Effect<string, never, never>
}

abstract class Cache {
  abstract get(key: string): Effect.Effect<string, never, never>
}

abstract class Database {
  abstract query(sql: string): Effect.Effect<string, never, never>
}

const UsersLive = Layer.succeed(Users, { find: (id: string) => Effect.succeed(`live-${id}`) })
const UsersTest = Layer.succeed(Users, { find: (id: string) => Effect.succeed(`test-${id}`) })

const CacheLive = Layer.effect(Cache, () => {
  const db = Database()
  return { get: (key: string) => db.query(key) }
})

const DatabaseLive = Layer.effect(Database, () => {
  const cache = Cache()
  return { query: (sql: string) => cache.get(sql) }
})

const UsersOpaque: Layer.Any = Layer.succeed(Users, { find: (id: string) => Effect.succeed(id) })

export const Ambiguous = wire(UsersLive, UsersTest)
export const Cyclic = wire(CacheLive, DatabaseLive)
export const Opaque = wire(UsersOpaque)
