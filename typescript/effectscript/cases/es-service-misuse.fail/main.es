// expect: ES3002 ES3003 ES3004 ES3005 ES3006
// A service is a key, not a base class (D1): extends (ES3002), new (ES3003),
// and instanceof (ES3004) are errors, a generic fully abstract class cannot
// be a service (ES3005), and a static member cannot share a service member's
// name (ES3006).
import { Effect } from "effect"

abstract class Users {
  abstract find(id: string): Effect.Effect<string, never, never>

  static find(id: string) {
    return id
  }
}

class UsersInMemory extends Users {}

abstract class Repository<T> {
  abstract get(id: string): Effect.Effect<T, never, never>
}

export function misuse(value: unknown) {
  const users = new Users()
  return value instanceof Users ? users : undefined
}
