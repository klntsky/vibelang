// Probe: what goes wrong if a plain abstract class is used as an Effect service identifier?
import { Context, Effect } from "effect"

// 1. Two "services" written as abstract classes with the same members.
abstract class Primary { abstract query(sql: string): Effect.Effect<string> }
abstract class Replica { abstract query(sql: string): Effect.Effect<string> }

// A key per class, as a compiler would generate one, but with the abstract class as the R identifier.
const PrimaryKey = Context.Service<Primary, { query(sql: string): Effect.Effect<string> }>("app/Primary")
const ReplicaKey = Context.Service<Replica, { query(sql: string): Effect.Effect<string> }>("app/Replica")

const both: Effect.Effect<string, never, Primary | Replica> = Effect.gen(function* () {
  const p = yield* PrimaryKey
  const r = yield* ReplicaKey
  return (yield* p.query("select 1")) + " / " + (yield* r.query("select 1"))
})

// Provide only Primary. Exclude<Primary | Replica, Primary> should leave Replica.
export const onlyPrimary = Effect.provideService(both, PrimaryKey, { query: (sql) => Effect.succeed(`primary ${sql}`) })
type Left = Effect.Services<typeof onlyPrimary>
export const leftIsNever: [Left] extends [never] ? "R collapsed to never" : "Replica still required" = "R collapsed to never"

// 2. The same with Effect's own class form: identical shapes, different keys.
class PrimaryS extends Context.Service<PrimaryS, { query(sql: string): Effect.Effect<string> }>()("app/PrimaryS") {}
class ReplicaS extends Context.Service<ReplicaS, { query(sql: string): Effect.Effect<string> }>()("app/ReplicaS") {}
type LeftS = Exclude<PrimaryS | ReplicaS, PrimaryS>
export const leftS: [LeftS] extends [never] ? "collapsed" : "ReplicaS still required" = "ReplicaS still required"

// 3. Abstract classes with a compiler-injected private brand.
abstract class PrimaryB { declare private readonly "~key": "app/PrimaryB"; abstract query(sql: string): Effect.Effect<string> }
abstract class ReplicaB { declare private readonly "~key": "app/ReplicaB"; abstract query(sql: string): Effect.Effect<string> }
type LeftB = Exclude<PrimaryB | ReplicaB, PrimaryB>
export const leftB: [LeftB] extends [never] ? "collapsed" : "ReplicaB still required" = "ReplicaB still required"

// 4. What TypeScript says today about the expressions a service call would use.
export function typeErrorsToday() {
  // @ts-expect-error  abstract instance member through the class: TS2339
  Primary.query("select 1")
  // @ts-expect-error  calling a class without new: TS2348
  Primary()
}

// Run the collapsed program: the type says nothing is missing.
Effect.runPromiseExit(onlyPrimary as Effect.Effect<string>).then((exit) => console.log("runtime:", exit._tag, exit._tag === "Failure" ? String(exit.cause).split("\n")[0] : exit.value))
