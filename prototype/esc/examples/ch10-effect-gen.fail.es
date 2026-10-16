// expect: ES1002
// Chapter 10 "Generator-style code stays in TypeScript": Effect.gen expects a JavaScript generator,
// but in EffectScript function* declares a producer.

class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}

interface User {
  readonly id: string
  readonly name: string
}

abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
}

const findName = (id: string) => Effect.gen(function* () {
  const users = yield* Users
  const user = yield* users.find(id)
  return user.name
})
