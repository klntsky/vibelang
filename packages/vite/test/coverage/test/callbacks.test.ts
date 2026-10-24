import { describe, expect, it } from "@effect/vitest"
import { Effect } from "effect"
import * as V from "../src/callbacks.vibe"
import * as T from "../src/callbacks.effect.ts"

for (const [name, m] of [["vibe", V], ["ts", T]] as const) {
  describe(name, () => {
    it.effect("runs a handler", () => Effect.gen(function* () {
      expect(yield* m.handler(1)(2)).toBe(3)
    }))
    it("builds a handler it never calls", () => {
      expect(typeof m.lazyHandler(1)).toBe("function")
    })
  })
}
