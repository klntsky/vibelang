import { Effect } from "effect"
let runs = 0
export const choose = (flag: boolean): Effect.Effect<string> | Effect.Effect<number> =>
  flag ? Effect.sync(() => { runs++; return "ok" }) : Effect.sync(() => { runs++; return 42 })
export const bootstrap = (flag: boolean): Effect.Effect<never, "refused"> | Effect.Effect<string[]> =>
  flag ? Effect.fail("refused" as const) : Effect.sync(() => { runs++; return ["--x"] })
export const counts = (): number => runs
