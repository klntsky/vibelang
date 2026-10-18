import { Effect } from "effect"

const double = (n: number) => Effect.succeed(n * 2)

export const doubled = Effect.forEach([1, 2, 3, 4], double, { concurrency: 2 })
