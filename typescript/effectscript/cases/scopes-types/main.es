// A Scope region supplies Scope to its using acquisitions and to the fibers
// it forks, so Scope leaves R only there (D14, D19, D47). contracts.ts checks
// each function's contract.
export interface Conn {
  readonly id: number
}

export class Down extends TaggedError("Down")<{}> {}

export abstract class Pool {
  abstract acquire(): Effect<Conn, Down, Scope>
}

export abstract class Audit {
  abstract record(event: string): Effect<void, never, never>
}

// using: the block's Scope; Pool and Down remain.
export function query() {
  using conn = Pool.acquire()
  return conn.id
}

// No using: the caller's Scope owns the connection.
export function lease() {
  const conn = Pool.acquire()
  return conn.id
}

// An acquisition without using and one with using in the same block.
export function both() {
  const kept = Pool.acquire()
  using held = Pool.acquire()
  return kept.id + held.id
}

// A block-owned fork adds its program's requirements, not Scope.
export function audited() {
  fork(() => Audit.record("start"))
  return "audited"
}

// A forked program that acquires without using still needs the caller's Scope.
export function forkedLease() {
  fork(() => lease())
  return "forked"
}

// With no block, fork is Effect.forkScoped: Scope stays in R.
export const background = () => fork(() => Audit.record("background"))

// Effect.forkScoped keeps Effect's contract inside a region too.
export function scopedFork() {
  using conn = Pool.acquire()
  Effect.forkScoped(() => Audit.record(`conn ${conn.id}`))
  return conn.id
}

let next = 0
const PoolTest = Layer.succeed(Pool, {
  acquire: () => Effect.acquireRelease(() => ({ id: ++next }), () => {})
})
const AuditTest = Layer.succeed(Audit, { record: () => {} })

function all() {
  const q = query()
  const b = Effect.scoped(() => both())
  const a = audited()
  return [q, b, a]
}

// expect-result: [1,5,"audited"]
export function main() {
  return Effect.provide(() => all(), Layer.mergeAll(PoolTest, AuditTest))
}
