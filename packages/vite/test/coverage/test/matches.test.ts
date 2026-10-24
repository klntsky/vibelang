import { describe, expect, it } from "@effect/vitest"
import { Effect } from "effect"
import * as V from "../src/matches.vibe"
import * as T from "../src/matches.effect.ts"

for (const [name, m] of [["vibe", V], ["ts", T]] as const) {
  describe(name, () => {
    it.effect("matches", () => Effect.gen(function* () {
      expect(m.label("draft")).toBe("D")
      expect(m.label("published")).toBe("P")
      expect(m.sized(0)).toBe("zero")
      expect(yield* m.describe(1)).toBe("some")
    }))
  })
}
