import { Effect } from "effect"

const user = Effect.succeed({ id: "1", name: "Ada" })
const orders = Effect.succeed([10, 20])

export const both = Effect.all([user, orders], { concurrency: "unbounded" }).pipe(
  Effect.map(([u, o]) => `${u.name} has ${o.length} orders`)
)
