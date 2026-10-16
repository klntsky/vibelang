import { assert, it } from "@effect/vitest"
import { EmptyName, greeting } from "./greeting.es"

it.effect("greets a name", () => {
  assert.strictEqual(greeting("Ada"), "Hello, Ada!")
})

it.effect("recovers from an empty name", () => {
  try {
    greeting("")
    assert.fail("expected EmptyName")
  } catch (error) {
    assert.instanceOf(error, EmptyName)
  }
})
