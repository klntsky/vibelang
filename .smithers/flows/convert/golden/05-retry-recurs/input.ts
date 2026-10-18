import { Data, Effect, Schedule } from "effect"

class Flaky extends Data.TaggedError("Flaky") {}

declare const fetchScore: Effect.Effect<number, Flaky>

export const score = fetchScore.pipe(Effect.retry(Schedule.recurs(3)))
