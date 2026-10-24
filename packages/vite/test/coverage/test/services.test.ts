import { describe, expect, it } from "@effect/vitest"
import { Effect } from "effect"
import * as V from "../src/services.vibe"
import * as T from "../src/services.effect.ts"

describe("vibe", () => {
  it.effect("finds a user", () => Effect.gen(function* () {
    expect(yield* V.nameOf("42")).toBe("Ada")
    expect(yield* V.greet("42")).toBe("hi Ada")
  }).pipe(Effect.provide(V.UsersLive)))
})
describe("ts", () => {
  it.effect("finds a user", () => Effect.gen(function* () {
    expect(yield* T.nameOf("42")).toBe("Ada")
    expect(yield* T.greet("42")).toBe("hi Ada")
  }).pipe(Effect.provide(T.UsersLive)))
})
