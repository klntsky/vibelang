// expect: TS2322 TS2339
import { Effect } from "effect"
export const value: number = "still an error"
export const oldName = Effect.catchAll
