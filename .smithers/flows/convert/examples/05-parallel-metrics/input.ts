import { Effect } from "effect"

const measure = (name: string) => Effect.sleep("3 millis").pipe(Effect.as(name.length))

export const metrics = Effect.gen(function* () {
  const [cpu, memory] = yield* Effect.all([measure("cpu"), measure("memory")], { concurrency: "unbounded" })
  const disks = yield* Effect.forEach(["root", "data", "backup"], measure, { concurrency: 8 })
  yield* Effect.log(`Measured ${disks.length} disks`)
  return { cpu, memory, disks }
})
