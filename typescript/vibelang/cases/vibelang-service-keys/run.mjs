// JavaScript uses the compiled service classes as Effect 4 Context keys.
import { Effect, Layer } from "effect"
import { Settings, Users, UsersLive, UsersTest } from "./out/main.js"

const greet = (id) =>
  Effect.gen(function* () {
    const settings = yield* Settings
    const user = yield* (yield* Users).find(id)
    return `${settings.greeting}, ${user.name}`
  })

console.log(Users.key, Settings.key)
console.log(await Effect.runPromise(Effect.provide(greet("1"), Layer.mergeAll(UsersLive, Settings.layer))))
console.log(await Effect.runPromise(Effect.provide(greet("7"), Layer.mergeAll(UsersTest, Settings.layer))))
const missing = await Effect.runPromise(Effect.flip(Effect.provide(greet("9"), Layer.mergeAll(UsersTest, Settings.layer))))
console.log(missing._tag, missing.id)
