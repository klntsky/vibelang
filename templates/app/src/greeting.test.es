import { assert, it } from "@effect/vitest"
import { EmptyName, greeting } from "./greeting.es"

it.effect("greets a name", () => {
  const actual = greeting("Ada")
  Effect.sync(() => assert.strictEqual(actual, "Hello, Ada!"))
})

it.effect("recovers from an empty name", () => {
  let recovered = false
  try {
    greeting("")
  } catch (error) {
    recovered = error instanceof EmptyName
  }
  Effect.sync(() => assert.strictEqual(recovered, true))
})
