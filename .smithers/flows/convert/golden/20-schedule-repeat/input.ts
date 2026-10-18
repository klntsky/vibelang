import { Effect, Schedule } from "effect"

export const ticks = Effect.log("tick").pipe(Effect.repeat(Schedule.recurs(2)))
