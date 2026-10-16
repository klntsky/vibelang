import { assert, it } from "@effect/vitest"
import { AppLive, greet, UsersTest } from "./main.es"

it.effect("uses the test layer", () => {
  const message = Effect.provide(() => greet("42"), UsersTest)
  Effect.sync(() => assert.strictEqual(message, "Welcome, Grace!"))
})

it.effect("recovers a typed missing-user failure", () => {
  const message = Effect.provide(() => greet("missing"), AppLive)
  Effect.sync(() => assert.strictEqual(message, "No user missing"))
})
