import { describe, expect, it } from "@effect/vitest"
import { Effect } from "effect"
import * as V from "../src/byname.vibe"
import * as T from "../src/byname.effect.ts"

for (const [name, m] of [["vibe", V], ["ts", T]] as const) {
  describe(name, () => {
    it.effect("runs programs passed by name", () => Effect.gen(function* () {
      expect(yield* m.timed(1)).toBe(1)
      expect(yield* m.pair(1)).toBe(3)
      expect(yield* m.retried(1)).toBe(1)
    }))
  })
}
