import { Context, Effect, Layer, Scope } from "effect"
import { $bind, $catch, $finally, TaggedError } from "../runtime/index.ts"

interface User { readonly id: string; readonly name: string; readonly email: string }
class UserNotFound extends TaggedError("UserNotFound")<{ readonly id: string }> {}
class MailError extends TaggedError("MailError")<{ readonly reason: string }> {}
class InvalidId extends TaggedError("InvalidId")<{ readonly raw: string }> {}

export class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, UserNotFound, never>
}>()("probe#Users") {}
export class Mailer extends Context.Service<Mailer, {
  send(to: string, body: string): Effect.Effect<void, MailError, never>
}>()("probe#Mailer") {}

// pass-1 shape
export const welcome1 = Effect.fnUntraced(function* (id: string) {
  const user = yield* $bind((yield* Users).find(id))
  yield* $bind((yield* Mailer).send(user.email, `Welcome, ${user.name}!`))
  return user.name
})
// final shape
export const welcome = Effect.fnUntraced(function* (id: string) {
  const user = yield* (yield* Users).find(id)
  yield* (yield* Mailer).send(user.email, `Welcome, ${user.name}!`)
  return user.name
})
export const parseId = Effect.fnUntraced(function* (raw: string) {
  if (!/^\d+$/.test(raw)) return yield* Effect.fail(new InvalidId({ raw }))
  return Number(raw)
})
const guest: User = { id: "0", name: "guest", email: "" }
export const lookupOrGuest = Effect.fnUntraced(function* (raw: string) {
  const $c = yield* $catch(Effect.fnUntraced(function* () {
    return { kind: "return" as const, value: yield* (yield* Users).find(String(yield* parseId(raw))) }
  })(), Effect.fnUntraced(function* (e) {
    if (e instanceof UserNotFound) return { kind: "return" as const, value: guest }
    return yield* Effect.fail(e)
  }))
  if ($c.kind === "return") return $c.value
  return undefined as never
})
// Layer with contextual typing through Effect.gen
export const UsersLive = Layer.effect(Users, Effect.gen(function* () {
  return {
    find(id) { return Effect.gen(function* () { if (id === "x") return yield* Effect.fail(new UserNotFound({ id })); return { id, name: "Ada", email: "a@x" } }) }
  }
}))
// alternative: arrow
export const UsersLive2 = Layer.effect(Users, Effect.gen(function* () {
  return { find: (id) => Effect.gen(function* () { return { id, name: "Ada", email: "a@x" } }) }
}))
type C<T> = T extends (...a: any) => Effect.Effect<infer A, infer E, infer R> ? [A, E, R] : never
export const c1: C<typeof welcome> = null!
export const c2: C<typeof lookupOrGuest> = null!
export const c3: C<typeof welcome1> = null!
