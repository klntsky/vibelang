import { Context, Effect, Layer } from "effect"
// Candidate D1 emission: the class name as a TYPE is the method shape (implements, parameter types),
// branded by an optional key so two services with the same members stay distinct in R.
type ServiceBase<I> = (abstract new () => {}) & Context.Key<I, I>
const base = <I>(key: string): ServiceBase<I> => Context.Service<I, I>()(key) as any

export interface Primary { query(sql: string): Effect.Effect<string>; readonly "~esc/service"?: "app/Primary" }
export class Primary extends base<Primary>("app/Primary") {
  static readonly layer = Layer.succeed(Primary, { query: (sql) => Effect.succeed(`primary ${sql}`) })
}
export interface Replica { query(sql: string): Effect.Effect<string>; readonly "~esc/service"?: "app/Replica" }
export class Replica extends base<Replica>("app/Replica") {}

// 1. Nominal R: providing Primary leaves Replica.
const both = Effect.gen(function* () {
  const p = yield* Primary
  const r = yield* Replica
  return (yield* p.query("1")) + (yield* r.query("1"))
})
export const onlyPrimary = Effect.provide(both, Primary.layer)
type Left = Effect.Services<typeof onlyPrimary>
export const left: [Left] extends [Replica] ? ([Replica] extends [Left] ? "Replica left" : "wrong") : "wrong" = "Replica left"
// 2. implements works, and a parameter typed with the service is the shape.
export class PrimaryFake implements Primary { query(sql: string) { return Effect.succeed(`fake ${sql}`) } }
export const useShape = (db: Primary) => db.query("x")
export const fakeLayer = Layer.succeed(Primary, new PrimaryFake())
// 3. Identical shapes are not interchangeable as R.
type Swap = Exclude<Primary | Replica, Primary>
export const swap: [Swap] extends [never] ? "collapsed" : "distinct" = "distinct"
const run = Effect.provide(both, Layer.mergeAll(Primary.layer, Layer.succeed(Replica, { query: (s) => Effect.succeed(`replica ${s}`) })))
Effect.runPromise(run).then((v) => console.log("runtime:", v, "| key:", (Primary as any).key))
