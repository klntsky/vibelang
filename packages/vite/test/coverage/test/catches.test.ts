import { describe, expect, it } from "@effect/vitest"
import { Effect } from "effect"
import * as V from "../src/catches.vibe"
import * as T from "../src/catches.effect.ts"

for (const [name, m] of [["vibe", V], ["ts", T]] as const) {
  describe(name, () => {
    it.effect("succeeds without mapping", () => Effect.gen(function* () {
      expect(yield* m.load(1)).toBe(2)
      expect(yield* m.loadDirect(1)).toBe(1)
      expect(yield* m.orZero(1)).toBe(1)
    }))
  })
}
