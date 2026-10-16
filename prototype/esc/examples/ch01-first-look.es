// Chapter 1. A first look: two services, a typed error, a function that uses them, and a main.
// NOTE: services are abstract classes (D1) and errors use the prelude TaggedError (D3), so the
// proposal's `import { Data } from "effect"` and `service` declarations are rewritten accordingly.

class UserNotFound extends TaggedError("UserNotFound")<{
  readonly id: string
}> {}

class MailError extends TaggedError("MailError")<{
  readonly reason: string
}> {}

interface User {
  readonly id: string
  readonly name: string
  readonly email: string
}

abstract class Users {
  abstract find(id: string): Effect<User, UserNotFound, never>
}

abstract class Mailer {
  abstract send(to: string, body: string): Effect<void, MailError, never>
}

export function welcome(id: string) {
  const user = Users.find(id)
  Mailer.send(user.email, `Welcome, ${user.name}!`)
  return user.name
}
// Effect<string, UserNotFound | MailError, Users | Mailer>

// Test providers in the style of Chapter 4: an object literal implements each service.
const UsersTest = Layer.succeed(Users, {
  find: (id) => ({ id, name: "Ada", email: "ada@example.com" })
})

const outbox: string[] = []

const MailerTest = Layer.succeed(Mailer, {
  send: (to, body) => {
    outbox.push(`${to}: ${body}`)
  }
})

const AppLive = Layer.mergeAll(UsersTest, MailerTest)

// NOTE: the proposal ends with `NodeRuntime.runMain(() => main())`; @effect/platform-node is not
// installed here, so the example runner runs main instead.
// expect-result: {"name":"Ada","outbox":["ada@example.com: Welcome, Ada!"]}
export function main() {
  const name = Effect.provide(() => welcome("42"), AppLive)
  Effect.log(`sent a welcome email to ${name}`)
  return { name, outbox }
}
